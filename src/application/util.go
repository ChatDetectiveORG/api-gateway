package app

import (
	"app/src/domain"
	"app/src/infrastructure/config"
	"app/src/infrastructure/rabbitmq"
	"context"
	"log"
	"regexp"
	"sync"

	e "app/pkg/errors"

	"github.com/gomodule/redigo/redis"
	amqp "github.com/rabbitmq/amqp091-go"

	redisDb "app/src/infrastructure/redis"
)

func calculaeType(ctx domain.Context) (updateType, *e.ErrorInfo) {
	update := ctx.Update()

	if update.Message != nil &&update.Message.Text != "" && regexp.MustCompile(`^/[a-zA-Z0-9_]{1,32}`).MatchString(update.Message.Text) {
		return slashCommand, e.Nil()
	}

	if update.Message != nil && update.Message.Text != "" {
		return textCommand, e.Nil()
	}

	if update.Callback != nil {
		return callbackQuery, e.Nil()
	}

	if update.EditedBusinessMessage != nil || update.DeletedBusinessMessages != nil {
		return businessEvent, e.Nil()
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
	if err != nil {
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
	if err != nil {
		return nil, e.FromError(err, "failed to get redis pool")
	}
	
	return pool.Get(), e.Nil()
}

func canBeHandledWithoutPatritions(updateType updateType) bool {
	return updateType == slashCommand || updateType == textCommand || updateType == businessConnectionChanged
}

func updateTypeToPodType(updateType updateType) handlerPodType {
	if updateType == slashCommand || updateType == textCommand || updateType == callbackQuery || updateType == businessConnectionChanged {
		return commandsAndQueries
	}

	switch updateType {
	case businessEvent:
		return  businessEvents
	case shipping:
		return shippingPods
	default:
		return ""
	}
}
