package main

import (
	"fmt"
	"net"
	"net/http"
	"runtime"
	"sync"
	"time"

	"github.com/4396/dht"
)

type DHTListener struct {
}

func newDHTListener() dht.Listener {
	return &DHTListener{}
}

var tid2 int64
var tors2 string

func (l *DHTListener) GetPeers(id *dht.ID, tor *dht.ID) {
	tid2++
	/*
		s := fmt.Sprintln("gp", tid2, tor)
		fmt.Print(s)
		tors2 = s + tors2
	*/
}

var tid int64
var tors string

func (l *DHTListener) AnnouncePeer(id *dht.ID, tor *dht.ID, peer *dht.Peer) {
	tid++
	s := fmt.Sprintln("ap", tid, tor)
	//fmt.Print(s)
	if tid%1000 == 0 {
		tors = ""
	}
	tors = s + tors
}

func newDHTServer() (d *dht.DHT, err error) {
	id := dht.NewRandomID()
	conn, err := net.ListenPacket("udp", ":0")
	if err != nil {
		return
	}
	handler := newDHTListener()
	d = dht.NewDHT(id, conn.(*net.UDPConn), 16, handler)
	return
}

func dhtNodeNums(d *dht.DHT) (n int) {
	d.Route().Map(func(b *dht.Bucket) bool {
		n += b.Count()
		return true
	})
	return
}

var routers = []string{
	"router.magnets.im:6881",
	"router.bittorrent.com:6881",
	"dht.transmissionbt.com:6881",
	"router.utorrent.com:6881",
}

func initDHTServer(d *dht.DHT) (err error) {
	for _, addr := range routers {
		addr, err := net.ResolveUDPAddr("udp", addr)
		if err != nil {
			break
		}
		err = d.FindNodeFromAddr(d.ID(), addr)
		if err != nil {
			break
		}
	}
	return
}

type udpMessage struct {
	idx  int
	addr *net.UDPAddr
	data []byte
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	msg := make(chan *udpMessage, 1024)
	var dhts []*dht.DHT
	var w sync.WaitGroup
	for i := 0; i < 1000; i++ {
		d, err := newDHTServer()
		if err != nil {
			continue
		}
		dhts = append(dhts, d)
		w.Add(1)
		go func(d *dht.DHT, idx int, msg chan *udpMessage) {
			defer w.Done()
			conn := d.Conn()
			buf := make([]byte, 1024)
			for {
				//buf := make([]byte, 1024)
				n, addr, err := conn.ReadFromUDP(buf)
				if err != nil {
					fmt.Println(err)
					continue
				}
				d.HandleMessage(addr, buf[:n])
				//msg <- &udpMessage{idx, addr, buf[:n]}
			}
		}(d, i, msg)
		if err = initDHTServer(d); err != nil {
			fmt.Println(err)
		}
	}

	go func() {
		checkup := time.Tick(time.Second * 30)
		cleanup := time.Tick(time.Minute * 15)
		serect := time.Tick(time.Minute * 15)

		for {
			select {
			case m := <-msg:
				d := dhts[m.idx]
				if m.addr != nil && m.data != nil {
					d.HandleMessage(m.addr, m.data)
				}
			case <-checkup:
				for _, d := range dhts {
					if d.NumNodes() < 1024 {
						d.FindNode(d.ID())
					}
				}
			case <-cleanup:
				for _, d := range dhts {
					d.CleanNodes(time.Minute * 15)
				}
			case <-serect:
				for _, d := range dhts {
					d.UpdateSecret()
				}
			default:
			}
		}
	}()

	go http.HandleFunc("/dht", func(res http.ResponseWriter, req *http.Request) {
		var count int
		for _, d := range dhts {
			n := dhtNodeNums(d)
			fmt.Print(n, " ")
			count += n
		}
		fmt.Println("//", count, tid, tid2)

		fmt.Fprintln(res, "===", count)
		//fmt.Fprintln(res, dhts[0].Route())
		fmt.Fprintln(res, "---------------------------------------------------")
		res.Write([]byte(tors))
		/*
			fmt.Fprintln(res, "---------------------------------------------------")
			res.Write([]byte(tors2))
		*/
	})
	http.ListenAndServe(":6882", nil)

	w.Wait()
}
