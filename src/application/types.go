package app

type updateType string

const (
	slashCommand              updateType = "slash_command"
	textCommand               updateType = "text_command"
	callbackQuery             updateType = "callback_query"
	businessEventNew          updateType = "business_event_new"
	businessEventEdited       updateType = "business_event_edited"
	shipping                  updateType = "shipping"
	businessConnectionChanged updateType = "business_connection_changed"
)

type handlerPodType string

const (
	commandsAndQueries   handlerPodType = "commands"
	businessEventsEdited handlerPodType = "business_events_edited"
	businessEventsNew    handlerPodType = "business_events_new"
	shippingPods         handlerPodType = "shipping"
)
