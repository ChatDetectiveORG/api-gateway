package app

import (
	"app/src/domain"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"sync"

	e "app/pkg/errors"

	"app/src/infrastructure/config"
	"app/src/infrastructure/rabbitmq"
	redisDb "app/src/infrastructure/redis"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/gomodule/redigo/redis"
)

var (
	apiUpdates chan(domain.Update)
	errors chan(*e.ErrorInfo)
)

func InitUpdateChabbel(cfg *config.Config, context context.Context, wg *sync.WaitGroup) {
	apiUpdates = make(chan(domain.Update), 1000)
	errors = make(chan(*e.ErrorInfo), 100)

	for range cfg.RuntimeConfig.NumRoutingGorutines {
		go routeUpdate(apiUpdates, errors, cfg, context, wg)
	}
}

func HandleApiUpdate(ctx domain.Update) {
	apiUpdates <- ctx
}

func routeUpdate(src chan(domain.Update), err chan(*e.ErrorInfo), cfg *config.Config, context context.Context, wg *sync.WaitGroup) {
	wg.Add(1)

	for {
		select {
		case <-context.Done():
			return
		case update := <-src:
			handleUpdate(update, err, cfg)
		}
	}
}

type updateType string

const (
	unknownUpdate  updateType = "unknown_update"
	slashCommand   updateType = "slash_command"
	textCommand    updateType = "text_command"
	callbackQuery  updateType = "callback_query"
	businessEvent  updateType = "business_event"
	shipping       updateType = "shipping"
	businessConnectionChanged updateType = "business_connection_changed"
)

func handleUpdate(update domain.Update, err chan(*e.ErrorInfo), cfg *config.Config) {
	updateType := calculaeType(update)

	if updateType == unknownUpdate {
		err <- e.NewError("unknown update type", "unknown update type").WithSeverity(e.Critical).WithData(map[string]any{
			"update": update,
		})
		return
	}

	destPod, funcErr := getDestPod(update, updateType, cfg)
	if funcErr != nil {
		err <- funcErr.PushStack()
		return
	}

	routingKey := fmt.Sprintf("%s:%s", destPod, updateType)
	content, marshalErr := json.Marshal(update)
	if marshalErr != nil {
		err <- e.FromError(marshalErr, "failed to marshal update")
		return
	}

	client, funcErr := rabbitmq.GetClient(cfg)
	if funcErr != nil {
		err <- funcErr.PushStack()
		return
	}
	ch, chErr := client.Channel()
	if chErr != nil {
		err <- e.FromError(chErr, "failed to get rabbitmq channel")
		return
	}
	defer func() { _ = ch.Close() }()

	publishErr := ch.PublishWithContext(
		context.Background(),
		"chatdetective.events",
		routingKey,
		false,
		false, 
		amqp.Publishing{
			ContentType: "application/json",
			Body: content,
		},
	)
	if publishErr != nil {
		err <- e.FromError(publishErr, "failed to publish update")
		return
	}
}

func getDestPod(update domain.Update, updateType updateType, cfg *config.Config) (string, *e.ErrorInfo) {
	if updateType == slashCommand || updateType == textCommand || updateType == businessConnectionChanged {
		leastLoadedPod, err := getLeastLoadedPod(cfg)
		if err != nil {
			return "", err.PushStack()
		}

		return leastLoadedPod, e.Nil()
	}

	var sessionID string

	switch updateType {
		case callbackQuery:
			sessionID = strconv.FormatInt(update.Callback.Message.Chat.ID, 10)

		case businessEvent:
			if update.EditedBusinessMessage != nil {
				sessionID = update.EditedBusinessMessage.BusinessConnectionID
			} else if update.DeletedBusinessMessages != nil {
				sessionID = update.DeletedBusinessMessages.BusinessConnectionID
			} else {
				sessionID = update.BusinessMessage.BusinessConnectionID
			}
	}

	pool, err := redisDb.GetPool(cfg)
	if err != nil {
		return "", e.FromError(err, "failed to get redis pool")
	}
	conn := pool.Get()
    defer conn.Close()
	pod, unwrappedError := redis.String(conn.Do("GET", fmt.Sprintf("sessions:%s", sessionID)))
	switch {
		case unwrappedError == redis.ErrNil:
			leastLoadedPod, err := getLeastLoadedPod(cfg)
			if err != nil {
				return "", err.PushStack()
			}

			_, unwrappedError = conn.Do("SET", fmt.Sprintf("sessions:%s", sessionID), leastLoadedPod)

			return leastLoadedPod, e.FromError(unwrappedError, "failed to set session")

		case unwrappedError != nil:
			return "", e.FromError(unwrappedError, "failed to get session")

		default:
			return pod, e.Nil()
	}
}

func getLeastLoadedPod(cfg *config.Config) (string, *e.ErrorInfo) {
	var routeScript = redis.NewScript(1, `
		local pod = redis.call("ZRANGE", KEYS[1], 0, 0)[1]
		if pod then
			redis.call("ZINCRBY", KEYS[1], 1, pod)
			return pod
		end
		return nil
	`)

	pool, err := redisDb.GetPool(cfg)
	if err != nil {
		return "", e.FromError(err, "failed to get redis pool")
	}
	conn := pool.Get()
    defer conn.Close()

	pod, unwrappedError := redis.String(routeScript.Do(conn, "pods:load"))
	if unwrappedError != nil {
		return "", e.FromError(unwrappedError, "failed to get least loaded pod")
	}
    
    return pod, e.Nil()
}

func calculaeType(update domain.Update) updateType {
	if regexp.MustCompile(`^/[a-zA-Z0-9_]{1,32}`).MatchString(update.Message.Text) {
		return slashCommand
	}

	if update.Message != nil && update.Message.Text != "" {
		return textCommand
	}

	if update.Callback != nil {
		return callbackQuery
	}

	if update.EditedBusinessMessage != nil || update.DeletedBusinessMessages != nil {
		return businessEvent
	}

	if update.ShippingQuery != nil || update.PreCheckoutQuery != nil {
		return shipping
	}

	return unknownUpdate
}

