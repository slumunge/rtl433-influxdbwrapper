package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/influxdata/influxdb/client/v2"
	"github.com/spf13/viper"
)

//{"time" : "2017-05-18 21:32:06", "brand" : "OS", "model" : "THGR122N", "id" : 250, "channel" : 1, "battery" : "OK", "temperature_C" : 21.400, "humidity" : 50}

type TempHygroSensorReading struct {
	//Time        time.Time //skip due to parse fuck
	Brand       string
	Model       string
	Id          int
	Channel     int
	Battery     string
	Temperature float32 `json:"temperature_C"`
	Humidity    int
}

func main() {
	fmt.Println("Starting")
	viper.SetDefault("influxdbaddress", "http://influxdb:8086")
	viper.SetDefault("database", "homesensors")
	viper.SetDefault("username", "")
	viper.SetDefault("password", "")
	viper.SetEnvPrefix("IWR")
	viper.BindEnv("influxdbaddress")
	viper.BindEnv("database")
	viper.BindEnv("username")
	viper.BindEnv("password")

	//setup database
	// Create a new HTTPClient
	c, err := client.NewHTTPClient(client.HTTPConfig{
		Addr:     viper.GetString("influxdbaddress"),
		Username: viper.GetString("username"),
		Password: viper.GetString("password"),
	})

	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	//make sure database exists
	q := client.NewQuery("CREATE DATABASE "+viper.GetString("database"), "", "")
	response, err := c.Query(q)
	if err != nil || response.Error() != nil {
		log.Fatal(err)
	}

	//setup and run command
	cmd := exec.Command("rtl_433", "-R", "12", "-F", "json", "-U")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}

	cmd.Stderr = os.Stderr
	fmt.Println("Starting command")
	printCommand(cmd)
	err = cmd.Start()
	if err != nil {
		log.Fatal(err)
	}

	in := bufio.NewScanner(stdout)
	var sensorReading TempHygroSensorReading
	for in.Scan() {

		err = json.Unmarshal(in.Bytes(), &sensorReading)
		if err != nil {
			log.Fatal(err)
		}

		bp, _ := client.NewBatchPoints(client.BatchPointsConfig{
			Database:  viper.GetString("database"),
			Precision: "s",
		})

		tags := map[string]string{
			"brand":    sensorReading.Brand,
			"model":    sensorReading.Model,
			"id":       strconv.Itoa(sensorReading.Id),
			"channel":  strconv.Itoa(sensorReading.Channel),
			"location": getLocation(sensorReading.Id, sensorReading.Channel),
		}
		fields := map[string]interface{}{
			"degree_celsius": sensorReading.Temperature,
		}

		pt, err := client.NewPoint("temperature", tags, fields, time.Now())
		if err != nil {
			log.Fatal(err)
		}
		bp.AddPoint(pt)

		fields = map[string]interface{}{
			"relative_humidity": sensorReading.Humidity,
		}
		pt, err = client.NewPoint("humidity", tags, fields, time.Now())
		if err != nil {
			log.Fatal(err)
		}
		bp.AddPoint(pt)

		fields = map[string]interface{}{
			"battery_status": sensorReading.Battery,
		}
		pt, err = client.NewPoint("battery", tags, fields, time.Now())
		if err != nil {
			log.Fatal(err)
		}
		bp.AddPoint(pt)

		err = c.Write(bp)
		if err != nil {
			log.Fatal(err)
		}

		log.Println(in.Text())
		log.Printf("%+v\n", sensorReading)
		//fmt.Println(in.Text())

	}
	if err != nil {
		log.Fatal(err)
	}

	cmd.Wait()
	log.Println("Finished")
}

func printCommand(cmd *exec.Cmd) {
	fmt.Printf("==> Executing: %s\n", strings.Join(cmd.Args, " "))
}

func getLocation(id int, channel int) string {

	location := "unknown"
	if id == 188 {
		location = "Outside"
	} else if id == 212 {
		location = "Basement"
	} else if id == 250 {
		location = "Bedroom"
	}
	return location
}

// func printError(err error) {
// 	if err != nil {
// 		os.Stderr.WriteString(fmt.Sprintf("==> Error: %s\n", err.Error()))
// 	}
// }
//
// func printOutput(outs []byte) {
// 	if len(outs) > 0 {
// 		fmt.Printf("==> Output: %s\n", string(outs))
// 	}
// }
