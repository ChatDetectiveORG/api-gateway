package app

import (
	"app/src/domain"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"

	e "app/pkg/errors"

	"app/src/infrastructure/config"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/gomodule/redigo/redis"
)

var (
	apiUpdates chan (domain.Context)
	errors     chan (*e.ErrorInfo)
)

func InitUpdateChannel(cfg *config.Config, context context.Context, wg *sync.WaitGroup) {
	apiUpdates = make(chan (domain.Context), 1000)
	errors = make(chan (*e.ErrorInfo), 100)

	go hanleError(errors, context, wg)

	for range cfg.RuntimeConfig.NumRoutingGorutines {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for {
				select {
				case <-context.Done():
					return
				case update := <-apiUpdates:
					handleUpdate(update, errors, cfg)
				}
			}
		}()
	}
}

func AddUpdateToChan(ctx domain.Context) error {
	apiUpdates <- ctx

	return nil
}

func handleUpdate(ctx domain.Context, errChan chan (*e.ErrorInfo), cfg *config.Config) {
	updateType, err := calculaeType(ctx)
	if !err.IsNil() {
		errChan <- err.PushStack()
		return
	}

	destPod, err := getDestPod(ctx, updateType, cfg)
	if err != nil {
		errChan <- err.PushStack()
		return
	}

	routingKey := fmt.Sprintf("%s:%s", destPod, updateType)

	content, unwrappedError := json.Marshal(ctx)
	if unwrappedError != nil {
		errChan <- e.FromError(unwrappedError, "failed to marshal update")
		return
	}

	rabbitmqChannel, err := newRabbitmqChannel(cfg)
	defer rabbitmqChannel.Close()

	unwrappedError = rabbitmqChannel.PublishWithContext(
		context.Background(),
		"chatdetective.events",
		routingKey,
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        content,
		},
	)
	if unwrappedError != nil {
		errChan <- e.FromError(unwrappedError, "failed to publish update")
		return
	}
}

func getSessionID(context domain.Context, updateType updateType) (string, *e.ErrorInfo) {
	update := context.Update()

	if updateType == callbackQuery {
		return strconv.FormatInt(update.Callback.Message.Chat.ID, 10), e.Nil()
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

func getDestPod(context domain.Context, updateType updateType, cfg *config.Config) (string, *e.ErrorInfo) {
	if canBeHandledWithoutPatritions(updateType) {
		leastLoadedPod, err := getLeastLoadedPod(cfg)
		if err != nil {
			return "", err.PushStack()
		}

		return leastLoadedPod, e.Nil()
	}

	sessionID, err := getSessionID(context, updateType)
	if !err.IsNil() {
		return "", err
	}

	redisConnection, err := newRedisConnection(cfg)
	if !err.IsNil() {
		return "", err
	}
	defer redisConnection.Close()
	pod, unwrappedError := redis.String(redisConnection.Do("GET", fmt.Sprintf("sessions:%s", sessionID)))

	switch {
	case unwrappedError == redis.ErrNil:
		leastLoadedPod, err := getLeastLoadedPod(cfg)
		if err != nil {
			return "", err.PushStack()
		}

		_, unwrappedError = redisConnection.Do("SET", fmt.Sprintf("sessions:%s", sessionID), leastLoadedPod, "EX", 600)
		if unwrappedError != nil {
			return "", e.FromError(unwrappedError, "failed to set session")
		}

		return leastLoadedPod, e.Nil()

	case unwrappedError != nil:
		return "", e.FromError(unwrappedError, "failed to get session")

	default:
		redisConnection.Do("EXPIRE", fmt.Sprintf("sessions:%s", sessionID), 600)

		return pod, e.Nil()
	}
}

// Получает наименее загруженный под
func getLeastLoadedPod(cfg *config.Config) (string, *e.ErrorInfo) {
	var routeScript = redis.NewScript(1, `
		local pod = redis.call("ZRANGE", KEYS[1], 0, 0)[1]
		if pod then
			redis.call("ZINCRBY", KEYS[1], 1, pod)
			return pod
		end
		return nil
	`)

	redisConnection, err := newRedisConnection(cfg)
	if !err.IsNil() {
		return "", err
	}
	defer redisConnection.Close()

	pod, unwrappedError := redis.String(routeScript.Do(redisConnection, "pods:load"))
	if unwrappedError != nil {
		return "", e.FromError(unwrappedError, "failed to get least loaded pod")
	}

	return pod, e.Nil()
}
