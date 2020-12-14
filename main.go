package main

import (
	"fmt"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/magiconair/properties"
	"github.com/sparrc/go-ping"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"encoding/json"
)

var p = properties.MustLoadFile("PROPERTIES", properties.UTF8)

var mqttclient mqtt.Client
// var mqtthost string
var brokername string
// var username string
// var password string

func main() {
	

	host := p.MustGetString("host")
	mqtthost := p.MustGetString("inetping_mqtt_host")
	brokername = p.MustGetString("inetping_mqtt_brokername")
	username := p.MustGetString("inetping_mqtt_username")
	password := p.MustGetString("inetping_mqtt_password")

	// listen for ctrl-C signal
	/*
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)
		go func() {
			for _ = range c {
				pinger.Stop()
			}
		}()
	*/
	// Init MQTT
	opts := mqtt.NewClientOptions().AddBroker(mqtthost)
	opts.SetUsername(username)
	opts.SetPassword(password)
	opts.SetClientID("golang-inetping") // Random client id
	opts.SetPingTimeout(10 * time.Second)
	opts.SetKeepAlive(10 * time.Second)
	opts.SetAutoReconnect(true)
	opts.SetMaxReconnectInterval(10 * time.Second)
	opts.SetConnectionLostHandler(func(c mqtt.Client, err error) {
		fmt.Printf("!!!!!! mqtt connection lost error: %s\n" + err.Error())
	})

	mqttclient = mqtt.NewClient(opts)
	if token := mqttclient.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error().Error())
	}


	for {
		pinger, err := ping.NewPinger(host)
		if err != nil {
			fmt.Printf("ERROR: %s\n", err.Error())
			return
		}

		pinger.Count = -1
		pinger.Interval = time.Millisecond * p.MustGetDuration("interval")
		pinger.Timeout = time.Millisecond * p.MustGetDuration("timeout")
		pinger.SetPrivileged(true)
		pinger.OnRecv = onReceived
		pinger.OnFinish = onFinish
		if err != nil {
			fmt.Printf("ERROR: %s\n", err.Error())
			return
		}
		fmt.Printf("PING %s (%s):\n", pinger.Addr(), pinger.IPAddr())
		pinger.Run()

	}
}

func onReceived(pkt *ping.Packet) {
	fmt.Printf("%d bytes from %s: icmp_seq=%d time=%v ttl=%v\n",
		pkt.Nbytes, pkt.IPAddr, pkt.Seq, pkt.Rtt, pkt.Ttl)
}

func onFinish(stats *ping.Statistics) {
	fmt.Printf("\n--- %s ping statistics ---\n", stats.Addr)
	fmt.Printf("%d packets transmitted, %d packets received, %v%% packet loss\n",
		stats.PacketsSent, stats.PacketsRecv, stats.PacketLoss)
	fmt.Printf("round-trip min/avg/max/stddev = %v/%v/%v/%v\n",
		stats.MinRtt, stats.AvgRtt, stats.MaxRtt, stats.StdDevRtt)

	isAlert := false
	client := resty.New()
	client.SetTimeout(time.Duration(10 * time.Second))

	// Evaluate Result here
	if stats.MaxRtt.Milliseconds() > p.MustGetInt64("maxlatency") {
		isAlert = true
		client.R().Get("http://192.168.1.84:9090/msg/maxlatency/" + fmt.Sprintf("%v", stats.MaxRtt))
		client.R().Get("http://192.168.1.84:9090/lcd/green")
	}

	if stats.PacketLoss > 0 {
		isAlert = true
		client.R().Get("http://192.168.1.84:9090/msg/packetloss/" + fmt.Sprintf("%v", stats.PacketLoss))
		client.R().Get("http://192.168.1.84:9090/lcd/red")
	}

	if !isAlert {
		client.R().Get("http://192.168.1.84:9090/clean")
	}

	// Publish MQTT payload
	// JSON marshall
	emp := make(map[string]interface{})
	emp["max_latency"] = stats.MaxRtt.Milliseconds() 
	emp["packet_loss"] = stats.PacketLoss
	empData, err := json.Marshal(emp)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	jsonStr := string(empData)
	mqttclient.Publish(brokername, 0, false, jsonStr)
}
