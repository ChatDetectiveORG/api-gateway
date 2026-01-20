package domain

import (
	tele "gopkg.in/telebot.v4"
)

// И так далее. Пока что зависим от telebot, в случае смены библиотеки всё равон используем интерфейсы оттуда
// Дальше как пойдёт

type Update = tele.Update

type Message = tele.Message
