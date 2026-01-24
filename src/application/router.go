package app

import (
	"app/src/domain"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

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
					handleUpdate(update, errors, cfg, rabbitmqChannel)
				}
			}
		}()
	}
}

func AddUpdateToChan(ctx domain.Context) error {
	apiUpdates <- ctx

	return nil
}

func handleUpdate(ctx domain.Context, errChan chan (*e.ErrorInfo), cfg *config.Config, rabbitmqChannel *amqp.Channel) {
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

	// TODOO: При добавлении зеркал продумать отправку данных о боте-источнике
	content, unwrappedError := json.Marshal(map[string]any{"update": ctx.Update()})
	if unwrappedError != nil {
		errChan <- e.FromError(unwrappedError, "failed to marshal update")
		return
	}

	poblishContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	unwrappedError = rabbitmqChannel.PublishWithContext(
		poblishContext,
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
	handlerPodT := updateTypeToPodType(updateType)
	if handlerPodT == "" {
		return "", e.NewError("Invalid update type", "Update type cannot be converted to handler pod type")
	}

	if canBeHandledWithoutPatritions(updateType) {
		leastLoadedPod, err := getLeastLoadedPod(cfg, handlerPodT)
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
		return getLeastLoadedPodAndWriteToCashe(cfg, redisConnection, sessionID, handlerPodT)

	case unwrappedError != nil:
		return "", e.FromError(unwrappedError, "failed to get session")

	default:
		loadKey := fmt.Sprintf("pods:handlers:%s:load", handlerPodT)
		_, ununwrappedError := redis.Float64(redisConnection.Do("ZSCORE", loadKey, pod))
		switch {
		case ununwrappedError == redis.ErrNil:
			// Под отсутствует в ZSET (или сам ZSET не существует) — перепривязываем сессию.
			return getLeastLoadedPodAndWriteToCashe(cfg, redisConnection, sessionID, handlerPodT)
		case ununwrappedError != nil:
			return "", e.FromError(ununwrappedError, fmt.Sprintf("error checking pod %s membership in redis zset %s", pod, loadKey))
		}

		redisConnection.Do("EXPIRE", fmt.Sprintf("sessions:%s", sessionID), 600)

		return pod, e.Nil()
	}
}

func getLeastLoadedPodAndWriteToCashe(cfg *config.Config, redisConnection redis.Conn, sessionID string, targetPodType handlerPodType) (string, *e.ErrorInfo) {
	leastLoadedPod, err := getLeastLoadedPod(cfg, targetPodType)
	if err != nil {
		return "", err.PushStack()
	}

	_, unwrappedError := redisConnection.Do("SET", fmt.Sprintf("sessions:%s", sessionID), leastLoadedPod, "EX", 600)
	if unwrappedError != nil {
		return "", e.FromError(unwrappedError, "failed to set session")
	}

	return leastLoadedPod, e.Nil()
}

// Получает наименее загруженный под
func getLeastLoadedPod(cfg *config.Config, targetPodType handlerPodType) (string, *e.ErrorInfo) {
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

	key := fmt.Sprintf("pods:handlers:%s:load", targetPodType)
	pod, unwrappedError := redis.String(routeScript.Do(redisConnection, key))

	// Если множество пустое или не существует, то ZRANGE вернёт nil, и
	// redis.String(routeScript.Do(...)) вернёт ошибку redis.ErrNil.
	if unwrappedError == redis.ErrNil {
		return "", e.NewError("no pods available for given pod type (set does not exist or is empty)", fmt.Sprintf("key: %s", key))
	}
	if unwrappedError != nil {
		return "", e.FromError(unwrappedError, "failed to get least loaded pod")
	}

	return pod, e.Nil()
}
