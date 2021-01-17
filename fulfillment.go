package main

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/golang/protobuf/jsonpb"
	"github.com/sirupsen/logrus"
	"google.golang.org/genproto/googleapis/cloud/dialogflow/v2"
)

func handleWebhook(c *gin.Context) {
	wReq := dialogflow.WebhookRequest{}

	if err := jsonpb.Unmarshal(c.Request.Body, &wReq); err != nil {
		logrus.WithError(err).Error("Couldn't Unmarshal request to jsonpb")
		c.Status(http.StatusBadRequest)
		return
	}
	fmt.Printf("%10s : %+v\n", "Intent", wReq.QueryResult.Intent.DisplayName)
	fmt.Printf("%10s : %+v\n", "Parameter", wReq.QueryResult.Parameters)
	fmt.Printf("%10s : %+v\n", "Text", wReq.QueryResult.QueryText)
	msg := ""
	params := wReq.QueryResult.Parameters.Fields
	switch wReq.QueryResult.Intent.DisplayName {
	case "開台詢問":
		if params["date-time"].GetStringValue() != "" {
			msg = fmt.Sprintf("Intent:%s\ndatetime:%s\nholoname:%s", wReq.QueryResult.Intent.DisplayName,
				params["date-time"].GetStringValue(),
				params["holoname"].GetStringValue())
		} else {
			datetime := struct {
				start string
				end   string
			}{}
			if params["date-time"].GetStructValue().Fields["startDate"].GetStringValue() != "" {
				datetime.start = params["date-time"].GetStructValue().Fields["startDate"].GetStringValue()
				datetime.end = params["date-time"].GetStructValue().Fields["endDate"].GetStringValue()
			} else if params["date-time"].GetStructValue().Fields["startDateTime"].GetStringValue() != "" {
				datetime.start = params["date-time"].GetStructValue().Fields["startDateTime"].GetStringValue()
				datetime.end = params["date-time"].GetStructValue().Fields["endDateTime"].GetStringValue()
			} else {
				datetime.start = params["date-time"].GetStructValue().Fields["startTime"].GetStringValue()
				datetime.end = params["date-time"].GetStructValue().Fields["endTime"].GetStringValue()
			}
			msg = fmt.Sprintf("Intent:%s\nholoname:%s\nfrom:\n%v\nto:\n%v",
				wReq.QueryResult.Intent.DisplayName,
				params["holoname"].GetStringValue(),
				datetime.start,
				datetime.end)
		}
	case "webhookDemo":
		msg = "2"
	default:
		msg = "err"
	}
	wRes := dialogflow.WebhookResponse{
		FulfillmentText: " OAOAOAOAO!!! ",
		FulfillmentMessages: []*dialogflow.Intent_Message{
			{
				Message: &dialogflow.Intent_Message_Text_{
					Text: &dialogflow.Intent_Message_Text{
						Text: []string{
							msg,
						},
					},
				},
			},
		},
	}
	m := jsonpb.Marshaler{}
	err := m.Marshal(c.Writer, &wRes)
	if err != nil {
		c.AbortWithError(http.StatusOK, err)
	}
}
