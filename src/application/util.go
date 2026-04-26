package app

import (
	"context"
	"log"
	"regexp"
	"sync"

	"github.com/ChatDetectiveORG/api-gateway/src/domain"
	"github.com/ChatDetectiveORG/api-gateway/src/infrastructure/config"
	"github.com/ChatDetectiveORG/api-gateway/src/infrastructure/rabbitmq"

	e "github.com/ChatDetectiveORG/shared/errors"

	"github.com/gomodule/redigo/redis"
	amqp "github.com/rabbitmq/amqp091-go"

	redisDb "github.com/ChatDetectiveORG/api-gateway/src/infrastructure/redis"
)

func calculaeType(ctx domain.Context) (updateType, *e.ErrorInfo) {
	update := ctx.Update()

	if update.Message != nil && update.Message.Text != "" && regexp.MustCompile(`^/[a-zA-Z0-9_]{1,32}`).MatchString(update.Message.Text) {
		return slashCommand, e.Nil()
	}

	if update.Message != nil && update.Message.Text != "" {
		return textCommand, e.Nil()
	}

	if update.Callback != nil {
		return callbackQuery, e.Nil()
	}

	if update.BusinessConnection != nil {
		return businessConnectionChanged, e.Nil()
	}

	if update.BusinessMessage != nil {
		return businessEventNew, e.Nil()
	}

	if update.EditedBusinessMessage != nil || update.DeletedBusinessMessages != nil {
		return businessEventEdited, e.Nil()
	}

	if update.ShippingQuery != nil || update.PreCheckoutQuery != nil {
		return shipping, e.Nil()
	}

	return "", e.NewError("Falid to calculate update type!", "")
}

func hanleError(src chan (*e.ErrorInfo), context context.Context, wg *sync.WaitGroup) {
	wg.Add(1)
	defer wg.Done()

	for {
		select {
		case <-context.Done():
			return
		case err := <-src:
			log.Println(err.JSON())
		}
	}
}

func newRabbitmqChannel(cfg *config.Config) (*amqp.Channel, *e.ErrorInfo) {
	client, err := rabbitmq.GetClient(cfg)
	if !err.IsNil() {
		return nil, err
	}

	ch, unwrappedError := client.Channel()
	if unwrappedError != nil {
		return nil, e.FromError(unwrappedError, "failed to get rabbitmq channel")
	}

	return ch, e.Nil()
}
func newRedisConnection(cfg *config.Config) (redis.Conn, *e.ErrorInfo) {
	pool, err := redisDb.GetPool(cfg)
	if !err.IsNil() {
		return nil, err
	}

	return pool.Get(), e.Nil()
}

func canBeHandledWithoutPatritions(updateType updateType) bool {
	return updateType == slashCommand || updateType == textCommand || updateType == businessConnectionChanged
}

func updateTypeToPodType(updateType updateType) handlerPodType {
	switch updateType {
	case slashCommand, textCommand, callbackQuery, businessConnectionChanged:
		return commandsAndQueries
	case businessEventNew:
		return businessEventsNew
	case businessEventEdited:
		return businessEventsEdited
	case shipping:
		return shippingPods
	default:
		return ""
	}
}
