package tclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"
)

type update struct {
	ID      int32   `json:"id"`
	Message Message `json:"message"`
}

type ChatSteward interface {
	HandleNewMessage(message Message)
}

type Controller struct {
	pipes          map[ChatID]chatPipe
	ctx            context.Context
	ticker         *time.Ticker
	stewardFactory func(Chat) ChatSteward
	endpoint       string
	botPrefix      string
	client         *http.Client
}

type options struct {
	tickInterval  time.Duration
	clientTimeout time.Duration
}

type Option func(*options)

func withTickInterval(interval time.Duration) Option {
	return func(o *options) {
		o.tickInterval = interval
	}
}

func withClientTimeout(timeout time.Duration) Option {
	return func(o *options) {
		o.clientTimeout = timeout
	}
}

func CreateController(ctx context.Context, endpoint string, token string, stewardFactory func(Chat) ChatSteward, opts ...Option) (*Controller, error) {
	p := options{
		tickInterval: 1 * time.Second,
	}

	for _, opt := range opts {
		opt(&p)
	}

	return &Controller{
		pipes:     make(map[ChatID]chatPipe),
		ctx:       ctx,
		ticker:    time.NewTicker(p.tickInterval),
		endpoint:  endpoint,
		botPrefix: "bot" + token,
		client: &http.Client{
			Timeout: time.Second,
		},
		stewardFactory: stewardFactory,
	}, nil
}

func sendRequest[T any](c *Controller, method string) (*T, error) {
	finalURL, err := url.JoinPath(c.endpoint, c.botPrefix, method)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Get(finalURL)
	if err != nil {
		return nil, err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("not successful code from telegram API on method %s (%d): %s", method, resp.StatusCode, string(body))
	}

	var res T
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, err
	}

	return &res, nil
}

func (c *Controller) getUpdates() ([]update, error) {
	updates, err := sendRequest[[]update](c, "getUpdates")
	if err != nil {
		return nil, err
	}
	if updates == nil {
		return nil, fmt.Errorf("got nil for updates without an error")
	}
	return *updates, nil
}

func (c *Controller) Start() {
	for {
		select {
		case <-c.ticker.C:
			updates, err := c.getUpdates()
			if err != nil {
				log.Printf("couldn't get updates:%s\n", err.Error())
				break
			}
			for _, upd := range updates {
				pipe, ok := c.pipes[upd.Message.Chat.ID]
				if !ok {
					pipe = chatPipe{
						steward: c.stewardFactory(upd.Message.Chat),
						updates: []update{},
						mx:      &sync.Mutex{},
						hasData: make(chan struct{}),
						ctx:     c.ctx,
					}
					c.pipes[upd.Message.Chat.ID] = pipe
					go pipe.routine()
				}
				pipe.mx.Lock()
				pipe.updates = append(pipe.updates, upd)
				pipe.mx.Unlock()
				select {
				case pipe.hasData <- struct{}{}:
				default:
				}
				continue
			}
		case <-c.ctx.Done():
			for _, pipe := range c.pipes {
				close(pipe.hasData)
			}
			return
		}
	}
}

type chatPipe struct {
	steward ChatSteward
	updates []update
	mx      *sync.Mutex
	hasData chan struct{}
	ctx     context.Context
}

func (p *chatPipe) routine() {
	for {
		select {
		case <-p.hasData:
			p.mx.Lock()
			for _, update := range p.updates {
				p.steward.HandleNewMessage(update.Message)
			}
			p.updates = []update{}
			p.mx.Unlock()
		case <-p.ctx.Done():
			return
		}
	}
}
