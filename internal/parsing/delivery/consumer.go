package delivery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/sirupsen/logrus"
	"github.com/streadway/amqp"
	"reminder_bot/internal/models"
	"strings"
	"time"
)

const (
	queueDurable    = true
	queueAutoDelete = false
	queueExclusive  = false
	queueNoWait     = false

	consumeAutoAck   = false
	consumeExclusive = false
	consumeNoLocal   = false
	consumeNoWait    = false
)

type ParsingConsumer struct {
	conn    *amqp.Connection
	channel *amqp.Channel
}

func NewParsingConsumer(amqpConn *amqp.Connection) (*ParsingConsumer, error) {
	ch, err := amqpConn.Channel()
	if err != nil {
		return nil, errors.New("error amqpConn.Channel")
	}
	return &ParsingConsumer{
		conn:    amqpConn,
		channel: ch,
	}, nil
}

func (c *ParsingConsumer) StartParseConsumer(ctx context.Context, workerPoolSize int, queueName string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	queue, err := c.channel.QueueDeclare(
		queueName,
		queueDurable,
		queueAutoDelete,
		queueExclusive,
		queueNoWait,
		nil,
	)

	if err != nil {
		return err
	}

	deliveries, err := c.channel.Consume(
		queue.Name,
		"",
		consumeAutoAck,
		consumeExclusive,
		consumeNoLocal,
		consumeNoWait,
		nil,
	)
	if err != nil {
		return err
	}

	for i := 0; i < workerPoolSize; i++ {
		go c.ParseWorker(ctx, deliveries)
	}
	select {
	case chanErr := <-c.channel.NotifyClose(make(chan *amqp.Error)):
		fmt.Printf("GetUserConsumer.Close: %v\n", chanErr)
		return err
	case <-ctx.Done():
		return nil
	}
}

func (c *ParsingConsumer) ParseWorker(ctx context.Context, messages <-chan amqp.Delivery) {
	for d := range messages {
		fmt.Println("New Request: ")
		req := &models.ParseRequest{}
		if err := json.Unmarshal(d.Body, req); err != nil {
			_ = d.Reject(false)
			fmt.Println(err)
			continue
		}
		fmt.Println("[ParseWorker]: \"", req.Text+"\"")
		var resp models.Response
		logrus.Println(req.Text)
		date, err := parse(req.Text)
		if err != nil {
			resp.IsSuccess = false
			resp.ErrText = err.Error()
		} else {
			resp.IsSuccess = true
		}

		textResp := models.ParseResponse{
			Text: req.Text,
			Time: date,
		}

		textRespJson, err := json.Marshal(textResp)
		if err != nil {
			fmt.Println(err)
		}
		resp.Body = textRespJson
		respData, err := json.Marshal(resp)
		if err != nil {
			panic(fmt.Sprintf("Failed marshall response: %v", err))
		}

		err = c.channel.Publish(
			"",
			d.ReplyTo,
			false,
			false,
			amqp.Publishing{
				ContentType:   d.ContentType,
				CorrelationId: d.CorrelationId,
				Body:          respData,
			})

		if err != nil {
			panic(fmt.Sprintf("Failed to publish a message: %v", err))
		}

		_ = d.Ack(false)
	}
}

func parse(str string) (time.Time, error) {
	str = strings.ToUpper(str)
	strArr := strings.Split(str, " ")
	for i, s := range strArr {
		if s == "??????????" {
			date, err := addDate(strArr[i+1], 1)
			if err == nil {
				return date, nil
			}
			number, bo := findNumber(strArr[i+1])
			if bo {
				date, err := addDate(strArr[i+2], number)
				return date, err
			}
		}
		if s == "??????????????" {
			d := time.Now().Add(time.Duration(60-time.Now().Minute()) * time.Minute)
			return d, nil
		}
		if s == "????????????" {
			d := time.Now().AddDate(0, 0, 1).Add(time.Duration(60-time.Now().Minute()) * time.Minute).Add(-1 * time.Hour)
			return d, nil
		}
	}
	return time.Now(), errors.New("???? ???????? ?????????????? ?????????? ???? ????????????, ?????????????????? ?????? ??????")
}

func addDate(s string, d int) (time.Time, error) {
	if strings.Contains(s, "??????") {
		return time.Now().Add(time.Duration(d) * time.Minute), nil
	}

	if strings.Contains(s, "??????") {
		return time.Now().Add(time.Duration(d) * time.Hour), nil
	}

	if strings.Contains(s, "??????????") {
		return time.Now().AddDate(0, d, 0), nil
	}

	if strings.Contains(s, "??????") {
		return time.Now().AddDate(1, 0, 0), nil
	}

	return time.Now(), errors.New("date not found")
}

func findNumber(s string) (int, bool) {
	logrus.Info(s)
	switch s {
	case "??????":
		return 2, true
	case "??????":
		return 2, true
	case "??????":
		return 3, true
	case "????????????":
		return 4, true
	case "????????":
		return 5, true
	case "??????????":
		return 6, true
	case "????????":
		return 7, true
	case "????????????":
		return 8, true
	case "????????????":
		return 9, true
	case "????????????":
		return 10, true
	default:
		return 0, false
	}
}
