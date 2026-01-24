package app

type updateType string

const (
	slashCommand   updateType = "slash_command"
	textCommand    updateType = "text_command"
	callbackQuery  updateType = "callback_query"
	businessEvent  updateType = "business_event"
	shipping       updateType = "shipping"
	businessConnectionChanged updateType = "business_connection_changed"
)

type handlerPodType string

const (
	commandsAndQueries handlerPodType = "commands"
	businessEvents     handlerPodType = "business_events"
	shippingPods       handlerPodType = "shipping"
)
