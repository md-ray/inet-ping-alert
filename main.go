package main

import (
	"fmt"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/magiconair/properties"
	"github.com/sparrc/go-ping"
)

var p = properties.MustLoadFile("PROPERTIES", properties.UTF8)

func main() {

	host := p.MustGetString("host")

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

	for {
		pinger, err := ping.NewPinger(host)
		if err != nil {
			fmt.Printf("ERROR: %s\n", err.Error())
			return
		}

		pinger.Count = -1
		pinger.Interval = time.Millisecond * p.MustGetInt("interval")
		pinger.Timeout = time.Millisecond * p.MustGetInt("timeout")
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
	if stats.MaxRtt.Milliseconds() > p.MustGetInt("maxlatency") {
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
}
