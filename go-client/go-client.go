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
	TempDs18B200   float64 `json:"temp_ds18b20_0"`
	TempDs18B201   float64 `json:"temp_ds18b20_1"`
	TempDs18B202   float64 `json:"temp_ds18b20_2"`
	TempDs18B203   float64 `json:"temp_ds18b20_3"`
	Rssi           int8    `json:"rssi"`
	Hdc1080Hum     float64 `json:"hdc_1080_hum"`
	Hdc1080Temp    float64 `json:"hdc_1080_temp"`
	Bmp280Temp     float64 `json:"bmp280_temp"`
	Bmp280Pressure float64 `json:"bmp280_pressure"`
	Altitude       int     `json:"altitude"`
	Timestamp      float64 `json:"timestamp"`
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
	if sample.Rssi != 0 {
		const insertStmt = `
		INSERT INTO measure(
			time,
			temp_ds18b20_0,
			temp_ds18b20_1,
			temp_ds18b20_2,
			temp_ds18b20_3,
			hdc_1080_hum,
			hdc_1080_temp,
			bmp280_temp,
			bmp280_pressure,
			altitude,
			rssi
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11);
		`

		_, qerr := db.Query(insertStmt,
			time,
			sample.TempDs18B200,
			sample.TempDs18B201,
			sample.TempDs18B202,
			sample.TempDs18B203,
			sample.Hdc1080Hum,
			sample.Hdc1080Temp,
			sample.Bmp280Temp,
			sample.Bmp280Pressure,
			sample.Altitude,
			sample.Rssi,
		)

		if qerr == nil {
			fmt.Printf("Add data to Pg: Time:%s Sample:%s\n", time, sample)
		} else {
			fmt.Printf("Sql Query fail:%s\n", qerr)
		}
	} else {
		const insertStmt = `
		INSERT INTO measure(
			time,
			temp_ds18b20_0,
			temp_ds18b20_1,
			temp_ds18b20_2,
			temp_ds18b20_3,
			hdc_1080_hum,
			hdc_1080_temp,
			bmp280_temp,
			bmp280_pressure,
			altitude
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10);
		`

		_, qerr := db.Query(insertStmt,
			time,
			sample.TempDs18B200,
			sample.TempDs18B201,
			sample.TempDs18B202,
			sample.TempDs18B203,
			sample.Hdc1080Hum,
			sample.Hdc1080Temp,
			sample.Bmp280Temp,
			sample.Bmp280Pressure,
			sample.Altitude,
		)

		if qerr != nil {
			fmt.Printf("Sql Query fail:%s\n", qerr)
		} else {
			fmt.Printf("Add sample No RSSI:%s\n", sample)
		}
	}
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

		data.Rssi = 0

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
