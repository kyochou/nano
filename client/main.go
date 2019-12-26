package client

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/gorilla/websocket"
	"github.com/lonng/nano/internal/codec"
	"github.com/lonng/nano/internal/message"
	"github.com/lonng/nano/internal/packet"
	"github.com/lonng/nano/tracing"
	opentracing "github.com/opentracing/opentracing-go"
)

var (
	hsd []byte // handshake data
	had []byte // handshake ack data
	hbd []byte
)

func init() {
	var err error
	hsd, err = codec.Encode(packet.Handshake, nil)
	if err != nil {
		panic(err)
	}

	had, err = codec.Encode(packet.HandshakeAck, nil)
	if err != nil {
		panic(err)
	}

	hbd, err = codec.Encode(packet.Heartbeat, nil)
	if err != nil {
		panic(err)
	}
}

type (

	// Callback represents the callback type which will be called
	// when the correspond events is occurred.
	Callback func(data []byte)

	// Connector is a tiny Nano client
	Connector struct {
		conn        *websocket.Conn // low-level connection
		tcpConn     *net.TCPConn    // low-level connection
		codec       *codec.Decoder  // decoder
		die         chan struct{}   // connector close channel
		chSend      chan []byte     // send queue
		mid         uint            // message id
		heartTicker *time.Ticker

		// events handler
		muEvents sync.RWMutex
		events   map[string]Callback

		// response handler
		muResponses sync.RWMutex
		responses   map[uint]Callback

		connectedCallback func() // connected callback
		opentraces        tracing.SmStrSpan
		Name              string
	}
)

// NewConnector create a new Connector
func NewConnector() *Connector {
	return &Connector{
		die:       make(chan struct{}),
		codec:     codec.NewDecoder(),
		chSend:    make(chan []byte, 64),
		mid:       1,
		events:    map[string]Callback{},
		responses: map[uint]Callback{},
	}
}

// Start connect to the server and send/recv between the c/s
func (c *Connector) Start(addr string) error {
	// log.Printf("connecting to %s", addr)
	wsConn, _, err := websocket.DefaultDialer.Dial(addr, nil)
	if err != nil {
		log.Fatal("dial:", err)
	}
	c.conn = wsConn

	tcpConn, ok := wsConn.UnderlyingConn().(*net.TCPConn)
	if ok {
		c.tcpConn = tcpConn
	} else {
		log.Fatal(`UnderlyingConn assert TCPConn error`)
	}

	go c.write()

	// send handshake packet
	c.send(hsd)

	// read and process network message
	go c.read()

	return nil
}

// OnConnected set the callback which will be called when the client connected to the server
func (c *Connector) OnConnected(callback func()) {
	c.connectedCallback = callback
}

// Request send a request to server and register a callbck for the response
func (c *Connector) Request(route string, v proto.Message, callback Callback) error {
	data, err := serialize(v)
	if err != nil {
		return err
	}

	msg := &message.Message{
		Type:  message.Request,
		Route: route,
		ID:    c.mid,
		Data:  data,
	}

	c.setResponseHandler(c.mid, callback)
	if err := c.sendMessage(msg); err != nil {
		c.setResponseHandler(c.mid, nil)
		return err
	}

	return nil
}

func (c *Connector) getNotifyMessage(route string, v proto.Message) (*message.Message, error) {
	data, err := serialize(v)
	if err != nil {
		return nil, err
	}

	return &message.Message{
		Type:  message.Notify,
		Route: route,
		Data:  data,
	}, nil
}

// Notify send a notification to server
func (c *Connector) Notify(route string, v proto.Message) error {
	msg, err := c.getNotifyMessage(route, v)
	if err != nil {
		return err
	}

	// log.Printf("send %v", msg)
	return c.sendMessage(msg)
}

func (c *Connector) TraceNotify(route string, v proto.Message, finishRoute string) error {
	msg, err := c.getNotifyMessage(route, v)
	if err != nil {
		return err
	}

	span := opentracing.GlobalTracer().StartSpan(fmt.Sprintf("%s => %s", route, finishRoute))
	span.SetTag(`req-route`, route)
	span.SetTag(`req-content`, v)
	c.opentraces.Store(finishRoute, span)

	b := bytes.NewBuffer([]byte{})
	span.Tracer().Inject(span.Context(), opentracing.Binary, b)
	ctx := b.Bytes()

	buf := make([]byte, 64)
	copy(buf[0:], []byte(finishRoute))
	b2 := make([]byte, 4)
	binary.LittleEndian.PutUint32(b2, uint32(len(ctx)))
	d := append(buf, b2...)
	d = append(d, ctx...)
	return c.sendTypedMessage(msg, packet.TraceData, d)
}

// On add the callback for the event
func (c *Connector) On(event string, callback Callback) {
	c.muEvents.Lock()
	defer c.muEvents.Unlock()

	c.events[event] = callback
}

// Close close the connection, and shutdown the benchmark
func (c *Connector) Close() {
	c.heartTicker.Stop()
	close(c.die)
	c.conn.Close()
}

func (c *Connector) eventHandler(event string) (Callback, bool) {
	c.muEvents.RLock()
	defer c.muEvents.RUnlock()

	cb, ok := c.events[event]
	return cb, ok
}

func (c *Connector) responseHandler(mid uint) (Callback, bool) {
	c.muResponses.RLock()
	defer c.muResponses.RUnlock()

	cb, ok := c.responses[mid]
	return cb, ok
}

func (c *Connector) setResponseHandler(mid uint, cb Callback) {
	c.muResponses.Lock()
	defer c.muResponses.Unlock()

	if cb == nil {
		delete(c.responses, mid)
	} else {
		c.responses[mid] = cb
	}
}

func (c *Connector) sendTypedMessage(msg *message.Message, typ packet.Type, extra []byte) error {
	data, err := msg.Encode()
	if err != nil {
		return err
	}

	if len(extra) > 0 {
		data = append(extra, data...)
	}
	payload, err := codec.Encode(typ, data)
	if err != nil {
		return err
	}

	c.mid++
	c.send(payload)

	return nil
}

func (c *Connector) sendMessage(msg *message.Message) error {
	return c.sendTypedMessage(msg, packet.Data, nil)
}

func (c *Connector) write() {
	defer close(c.chSend)

	for {
		select {
		case data := <-c.chSend:
			// log.Printf("send: %#v\n", data)
			if err := c.conn.WriteMessage(websocket.BinaryMessage, data); err != nil {
				log.Println(err.Error())
				c.Close()
			}

		case <-c.die:
			return
		}
	}
}

func (c *Connector) send(data []byte) {
	// c.tcpConn.SetNoDelay(true)
	c.chSend <- data
}

func (c *Connector) read() {
	for {
		_, buf, err := c.conn.ReadMessage()
		if err != nil {
			// log.Println(err.Error())
			c.Close()
			return
		}

		packets, err := c.codec.Decode(buf)
		if err != nil {
			log.Println(err.Error())
			c.Close()
			return
		}

		for i := range packets {
			p := packets[i]
			c.processPacket(p)
		}
	}
}

func (c *Connector) processPacket(p *packet.Packet) {
	// fmt.Printf("packet: %#v\n", p)
	switch p.Type {
	case packet.Handshake:
		c.send(had)
		var data map[string]interface{}
		if err := json.Unmarshal(p.Data, &data); err != nil {
			panic(err)
		}
		if data[`code`].(float64) != 200 {
			panic(`code not 200`)
		}

		sys, _ := data[`sys`].(map[string]interface{})
		i := time.Duration(sys[`heartbeat`].(float64))
		c.heartTicker = time.NewTicker(i * time.Second)
		go func() {
			for {
				<-c.heartTicker.C
				c.send(hbd)
			}
		}()

		c.connectedCallback()
	case packet.Data:
		msg, err := message.Decode(p.Data)
		if err != nil {
			log.Println(err.Error())
			return
		}
		c.processMessage(msg)

	case packet.Kick:
		c.Close()
	}
}

func (c *Connector) processMessage(msg *message.Message) {
	// log.Printf("on %v", msg)
	switch msg.Type {
	case message.Push:
		cb, ok := c.eventHandler(msg.Route)
		if !ok {
			// log.Println("event handler not found", msg.Route)
			return
		}
		// log.Printf("on %v", msg)
		if opspan, ok := c.opentraces.Load(msg.Route); ok {
			opspan.Finish()
		}
		cb(msg.Data)

	case message.Response:
		cb, ok := c.responseHandler(msg.ID)
		if !ok {
			// log.Println("response handler not found", msg.ID)
			return
		}
		// log.Printf("on %v", msg)
		cb(msg.Data)
		c.setResponseHandler(msg.ID, nil)
	}
}

func serialize(v proto.Message) ([]byte, error) {
	data, err := proto.Marshal(v)
	if err != nil {
		return nil, err
	}
	return data, nil
}
