package rtmkt

import (
	"bytes"
	"context"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

//go:generate cbor-gen-for --map-encoding GossipMessage

const PinsTopic = "/myel/pins"

type GossipMessage struct {
	PayloadCID cid.Cid
	SenderID   peer.ID
}

// GossipChannel represents a subscription to a topic
type GossipChannel struct {
	Messages chan *GossipMessage

	ctx   context.Context
	ps    *pubsub.PubSub
	topic *pubsub.Topic
	sub   *pubsub.Subscription

	self peer.ID
}

type Gossip struct {
	Pins *GossipChannel

	ps *pubsub.PubSub
}

func NewGossip(ctx context.Context, h host.Host) (*Gossip, error) {
	ps, err := pubsub.NewGossipSub(ctx, h)
	if err != nil {
		return nil, err
	}
	pins, err := NewGossipChannel(ctx, PinsTopic, ps, h.ID())
	if err != nil {
		return nil, err
	}
	g := &Gossip{
		Pins: pins,
		ps:   ps,
	}
	return g, nil
}

func NewGossipChannel(ctx context.Context, topicName string, ps *pubsub.PubSub, selfID peer.ID) (*GossipChannel, error) {
	topic, err := ps.Join(topicName)
	if err != nil {
		return nil, err
	}

	sub, err := topic.Subscribe()
	if err != nil {
		return nil, err
	}

	c := &GossipChannel{
		ctx:      ctx,
		ps:       ps,
		sub:      sub,
		self:     selfID,
		topic:    topic,
		Messages: make(chan *GossipMessage),
	}
	go c.readLoop()
	return c, nil
}

// readLoop pulls messages from the pubsub topic and pushes them onto the Messages channel.
func (c *GossipChannel) readLoop() {
	for {
		msg, err := c.sub.Next(c.ctx)
		if err != nil {
			close(c.Messages)
			return
		}
		// only forward messages delivered by others
		if msg.ReceivedFrom == c.self {
			continue
		}
		m := new(GossipMessage)
		if err := m.UnmarshalCBOR(bytes.NewReader(msg.Data)); err != nil {
			continue
		}
		// send valid messages onto the Messages channel
		c.Messages <- m
	}
}

// Publish sends new content to the pubsub topic
func (c *GossipChannel) Publish(payload cid.Cid) error {
	m := GossipMessage{
		PayloadCID: payload,
		SenderID:   c.self,
	}
	buf := new(bytes.Buffer)
	if err := m.MarshalCBOR(buf); err != nil {
		return err
	}
	return c.topic.Publish(c.ctx, buf.Bytes())
}
