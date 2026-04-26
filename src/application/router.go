package app

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/ChatDetectiveORG/api-gateway/src/domain"

	e "github.com/ChatDetectiveORG/shared/errors"

	"github.com/ChatDetectiveORG/api-gateway/src/infrastructure/config"

	amqp "github.com/rabbitmq/amqp091-go"
)

var (
	apiUpdates chan (domain.Context)
	errors     chan (*e.ErrorInfo)
)

const shardCount = 64

func InitUpdateChannel(cfg *config.Config, context context.Context, wg *sync.WaitGroup) {
	apiUpdates = make(chan (domain.Context), 10000)
	errors = make(chan (*e.ErrorInfo), 1000)

	go hanleError(errors, context, wg)

	for i := 0; i < cfg.RuntimeConfig.NumRoutingGorutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			rabbitmqChannel, err := newRabbitmqChannel(cfg)
			if !err.IsNil() {
				errors <- err.PushStack()
				return
			}
			defer rabbitmqChannel.Close()

			for {
				select {
				case <-context.Done():
					return
				case update := <-apiUpdates:
					handleUpdate(update, errors, rabbitmqChannel)
				}
			}
		}()
	}
}

func AddUpdateToChan(ctx domain.Context) error {
	update := ctx.Update()
	if update.PreCheckoutQuery != nil {
		log.Printf("received precheckout update id=%s payload=%s", update.PreCheckoutQuery.ID, update.PreCheckoutQuery.Payload)
	}
	apiUpdates <- ctx

	return nil
}

func handleUpdate(ctx domain.Context, errChan chan (*e.ErrorInfo), rabbitmqChannel *amqp.Channel) {
	updateType, err := calculaeType(ctx)
	if !err.IsNil() {
		errChan <- err.PushStack()
		return
	}

	sessionID, err := getSessionID(ctx, updateType)
	if !err.IsNil() {
		errChan <- err.PushStack()
		return
	}

	podType := updateTypeToPodType(updateType)
	if podType == "" {
		errChan <- e.NewError("unknown pod type for update type: "+string(updateType), "")
		return
	}

	routingKey := string(podType) + "." + shardRoutingKey(sessionID)
	if updateType == shipping {
		log.Printf("publishing payment update type=%s session=%s rk=%s", updateType, sessionID, routingKey)
	}

	// TODOO: При добавлении зеркал продумать отправку данных о боте-источнике
	content, unwrappedError := json.Marshal(ctx.Update())
	if unwrappedError != nil {
		errChan <- e.FromError(unwrappedError, "failed to marshal update")
		return
	}

	traceID := fmt.Sprintf("%s-%d", routingKey, time.Now().UnixNano())

	poblishContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	unwrappedError = rabbitmqChannel.PublishWithContext(
		poblishContext,
		"chatdetective.events",
		routingKey,
		false,
		false,
		amqp.Publishing{
			CorrelationId: traceID,
			MessageId:     traceID,
			ContentType:   "application/json",
			Headers: amqp.Table{
				"session_id":  sessionID,
				"update_type": string(updateType),
			},
			Body: content,
		},
	)
	if unwrappedError != nil {
		errChan <- e.FromError(unwrappedError, "failed to publish update")
		return
	}

	log.Printf("trace=%s published exchange=%s rk=%s", traceID, "chatdetective.events", routingKey)
}

func shardRoutingKey(sessionID string) string {
	// Создаем новый 32-битный хешер типа FNV-1a
	h := fnv.New32a()
	// Преобразуем строку sessionID в срез байт и записываем его в хешер
	_, _ = h.Write([]byte(sessionID))
	// Получаем числовое значение хеша, берём остаток от деления на количество шард (shardCount)
	// таким образом равномерно распределяем сессии по шардам, а затем приводим к типу int
	shard := int(h.Sum32() % uint32(shardCount))
	// Возвращаем имя очереди в виде строки "qXX", где XX — номер шарда с ведущим нулём если он меньше 10
	return fmt.Sprintf("q%02d", shard)
}

func getSessionID(context domain.Context, updateType updateType) (string, *e.ErrorInfo) {
	update := context.Update()

	// For regular chats (commands / text), session is the chat id.
	if update.Message != nil && update.Message.Chat != nil && (updateType == slashCommand || updateType == textCommand) {
		return strconv.FormatInt(update.Message.Chat.ID, 10), e.Nil()
	}

	if updateType == callbackQuery {
		if update.Callback == nil {
			return "", e.NewError("Callback is nil", "Callback is nil")
		}
		if update.Callback.Message == nil {
			return "", e.NewError("Message is nil", "Message is nil")
		}
		if update.Callback.Message.Chat == nil {
			return "", e.NewError("Chat is nil", "Chat is nil")
		}

		return strconv.FormatInt(update.Callback.Message.Chat.ID, 10), e.Nil()
	}

	if updateType == businessConnectionChanged {
		if update.BusinessConnection == nil {
			return "", e.NewError("BusinessConnection is nil", "BusinessConnection is nil")
		}
		if update.BusinessConnection.ID == "" {
			return "", e.NewError("BusinessConnection id is empty", "BusinessConnection id is empty")
		}
		return update.BusinessConnection.ID, e.Nil()
	}

	if updateType == shipping {
		if update.PreCheckoutQuery != nil && update.PreCheckoutQuery.Sender != nil {
			return strconv.FormatInt(update.PreCheckoutQuery.Sender.ID, 10), e.Nil()
		}
		if update.ShippingQuery != nil && update.ShippingQuery.Sender != nil {
			return strconv.FormatInt(update.ShippingQuery.Sender.ID, 10), e.Nil()
		}
		return "", e.NewError("No valid payment session token in update", "Invalid payment update!")
	}

	if update.EditedBusinessMessage != nil {
		return update.EditedBusinessMessage.BusinessConnectionID, e.Nil()
	} else if update.DeletedBusinessMessages != nil {
		return update.DeletedBusinessMessages.BusinessConnectionID, e.Nil()
	} else if update.BusinessMessage != nil {
		return update.BusinessMessage.BusinessConnectionID, e.Nil()
	}

	return "", e.NewError("No valid session token in update", "Invalid update!")
}
