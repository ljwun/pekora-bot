package main

import (
	"fmt"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func main() {
	var port = os.Getenv("PORT")
	var err error

	r := gin.Default()
	r.POST("/webhook", handleWebhook)
	r.POST("/bot/text", handleBotText)
	r.POST("/bot/voice", handleBotVoice)

	if err = r.Run(fmt.Sprint(":", port)); err != nil {
		logrus.WithError(err).Fatal("Couldn't start server")
	}
	logrus.Println("server running on port ", port)
}
