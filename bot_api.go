package main

import (
	"fmt"
	"net/http"

	"context"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	dialogflow "cloud.google.com/go/dialogflow/apiv2"
	dialogflowpb "google.golang.org/genproto/googleapis/cloud/dialogflow/v2"
)

const (
	projectID    string = "devbot-shwx"
	languageCode string = "zh-TW"
)

func handleBot(c *gin.Context) {
	botSession := BotSession{}
	if err := c.ShouldBindJSON(&botSession); err != nil {
		logrus.Println(err)
		c.AbortWithError(http.StatusBadRequest, err)
	}
	sessionID := botSession.SessionID
	var (
		text             string
		messages         []string
		err              error
		dialogFlowClient = DialogFlowClient{
			SessionID:    sessionID,
			ProjectID:    projectID,
			LanguageCode: languageCode,
		}
	)
	switch botSession.RequestType {
	case "Text":
		text, messages, err = dialogFlowClient.DetectIntentWithText(context.Background(), botSession.RequestText)
	case "Audio":
		text, messages, err = dialogFlowClient.DetectIntentWithAudio(context.Background(), botSession.RequestAudio)
	}
	if err != nil {
		logrus.Println(err)
		c.AbortWithError(http.StatusBadRequest, err)
	}
	c.JSON(http.StatusOK, gin.H{
		"sessionID":    sessionID,
		"languageCode": languageCode,
		"response": gin.H{
			"text":     text,
			"messages": messages,
		},
	})
}

// BotSession is data change between bot and api
type BotSession struct {
	SessionID    string `json:"sessionID"`
	RequestType  string `json:"requestType"`
	RequestAudio []byte `json:"voice"`
	RequestText  string `json:"text"`
}

type DialogFlowClient struct {
	SessionID    string
	ProjectID    string
	LanguageCode string
}

func (c *DialogFlowClient) DetectIntentWithText(ctx context.Context, text string) (string, []string, error) {
	request := dialogflowpb.DetectIntentRequest{
		QueryInput: &dialogflowpb.QueryInput{
			Input: &dialogflowpb.QueryInput_Text{
				Text: &dialogflowpb.TextInput{
					Text:         text,
					LanguageCode: c.LanguageCode,
				},
			},
		},
	}
	return c.detectIntent(ctx, &request)
}

func (c *DialogFlowClient) DetectIntentWithAudio(ctx context.Context, audioBytes []byte) (string, []string, error) {
	request := dialogflowpb.DetectIntentRequest{
		QueryInput: &dialogflowpb.QueryInput{
			Input: &dialogflowpb.QueryInput_AudioConfig{
				AudioConfig: &dialogflowpb.InputAudioConfig{
					AudioEncoding:   dialogflowpb.AudioEncoding_AUDIO_ENCODING_OGG_OPUS,
					SampleRateHertz: 16000,
					LanguageCode:    languageCode,
				},
			},
		},
		InputAudio: audioBytes,
	}
	return c.detectIntent(ctx, &request)
}

func (c *DialogFlowClient) detectIntent(ctx context.Context, request *dialogflowpb.DetectIntentRequest) (string, []string, error) {
	sessionClient, err := dialogflow.NewSessionsClient(ctx)
	if err != nil {
		return "", []string{}, err
	}
	defer sessionClient.Close()

	if projectID == "" || c.SessionID == "" {
		return "", []string{}, fmt.Errorf("received empty project (%s) or session (%s)", projectID, c.SessionID)
	}
	request.Session = fmt.Sprintf("projects/%s/agent/sessions/%s", projectID, c.SessionID)
	response, err := sessionClient.DetectIntent(ctx, request)
	if err != nil {
		return "", []string{}, err
	}

	queryResult := response.GetQueryResult()
	fmt.Printf("Query : %+v\n", queryResult.QueryText)
	fulfillmentText := queryResult.GetFulfillmentText()
	fulfillmentMessages := queryResult.FulfillmentMessages[0].GetText().Text
	return fulfillmentText, fulfillmentMessages, nil
}
