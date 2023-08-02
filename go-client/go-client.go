package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"time"

	_ "github.com/lib/pq"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type SampleSt struct {
	TempDs18B201   float64 `json:"temp_ds18b20_1"`
	TempDs18B200   float64 `json:"temp_ds18b20_0"`
	Hdc1080Hum     float64 `json:"hdc_1080_hum"`
	Hdc1080Temp    float64 `json:"hdc_1080_temp"`
	Bmp280Temp     float64 `json:"bmp280_temp"`
	Bmp280Pressure float64 `json:"bmp280_pressure"`
	Altitude       int     `json:"altitude"`
	Timestamp      float64 `json:"timestamp"`
}

func create_table() {
	user := os.Getenv("PG_USER")
	password := os.Getenv("PG_PASS")
	host := os.Getenv("PG_HOST")
	dbname := os.Getenv("PG_DB")

	psqlconn := fmt.Sprintf("host=%s port=5432 user=%s password=%s dbname=%s sslmode=disable", host, user, password, dbname)

	db, _ := sql.Open("postgres", psqlconn)

	q := `
	CREATE EXTENSION IF NOT EXISTS timescaledb CASCADE;

	CREATE TABLE IF NOT EXISTS measure (
		time TIMESTAMPTZ NOT NULL,
		temp_ds18b20_1 DOUBLE PRECISION NULL,
		temp_ds18b20_0 DOUBLE PRECISION NULL,
		hdc_1080_hum DOUBLE PRECISION NULL,
		hdc_1080_temp DOUBLE PRECISION NULL,
		bmp280_temp DOUBLE PRECISION NULL,
		bmp280_pressure DOUBLE PRECISION NULL,
		altitude DOUBLE PRECISION NULL
	);
	SELECT CREATE_HYPERTABLE IF NOT EXISTS('measure', 'time');
	`

	_, err := db.Exec(q)
	if err != nil {
		fmt.Println("Error can't create table", err)
	} else {
		fmt.Println("Table Okay")
	}
}

func insert_data_in_db(sample SampleSt) {
	user := os.Getenv("PG_USER")
	password := os.Getenv("PG_PASS")
	host := os.Getenv("PG_HOST")
	dbname := os.Getenv("PG_DB")

	psqlconn := fmt.Sprintf("host=%s port=5432 user=%s password=%s dbname=%s sslmode=disable", host, user, password, dbname)

	db, err := sql.Open("postgres", psqlconn)
	if err != nil {
		fmt.Printf("Sql open fail:%s\n", err)
		return
	}
	defer db.Close()

	time := time.UnixMilli(int64(sample.Timestamp * 1000))

	const insertStmt = `
INSERT INTO measure(
	time,
	temp_ds18b20_0,
	temp_ds18b20_1,
	hdc_1080_hum,
	hdc_1080_temp,
	bmp280_temp,
	bmp280_pressure,
	altitude
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8);
`
	fmt.Println(insertStmt, "as")
	res, qerr := db.Query(insertStmt,
		time,
		sample.TempDs18B200,
		sample.TempDs18B201,
		sample.Hdc1080Hum,
		sample.Hdc1080Temp,
		sample.Bmp280Temp,
		sample.Bmp280Pressure,
		sample.Altitude)

	if qerr != nil {
		fmt.Printf("Sql Query fail:%s\n", qerr)
	}

	fmt.Println("Add data to Pg:", res)
}

func createClientOptions(clientId string, uri *url.URL) *mqtt.ClientOptions {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tcp://%s", uri.Host))
	opts.SetUsername(uri.User.Username())
	password, _ := uri.User.Password()
	opts.SetPassword(password)
	opts.SetClientID(clientId)
	return opts
}

func try_run() error {
	uri, err := url.Parse(os.Getenv("MQTT_URI"))
	if err != nil {
		log.Fatal(err)
	}

	topic := "wheater/sample"

	create_table()

	opts := createClientOptions("sub", uri)
	client := mqtt.NewClient(opts)
	token := client.Connect()
	for !token.WaitTimeout(3 * time.Second) {
	}
	if err := token.Error(); err != nil {
		return err
	}

	client.Subscribe(topic, 0, func(client mqtt.Client, msg mqtt.Message) {
		data := SampleSt{}

		unmarshal_err := json.Unmarshal(msg.Payload(), &data)
		if unmarshal_err != nil {
			fmt.Printf("UnMarshall error! %s\n", unmarshal_err)
		}

		insert_data_in_db(data)
	})

	/* Wait messagges */
	for {
		time.Sleep(1000)
	}
}

func main() {
	for {
		err := try_run()
		fmt.Println("Error", err)
		time.Sleep(1000)
	}
}
