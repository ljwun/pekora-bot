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

//DetectIntentText : Detect Intent via text and get text message from dialogflow
func DetectIntentText(projectID, sessionID, languageCode, text string) (string, []string, error) {
	ctx := context.Background()

	sessionClient, err := dialogflow.NewSessionsClient(ctx)
	if err != nil {
		return "", []string{}, err
	}
	defer sessionClient.Close()

	if projectID == "" || sessionID == "" {
		return "", []string{}, fmt.Errorf("Received empty project (%s) or session (%s)", projectID, sessionID)
	}

	sessionPath := fmt.Sprintf("projects/%s/agent/sessions/%s", projectID, sessionID)
	textInput := dialogflowpb.TextInput{Text: text, LanguageCode: languageCode}
	queryTextInput := dialogflowpb.QueryInput_Text{Text: &textInput}
	queryInput := dialogflowpb.QueryInput{Input: &queryTextInput}
	request := dialogflowpb.DetectIntentRequest{Session: sessionPath, QueryInput: &queryInput}

	response, err := sessionClient.DetectIntent(ctx, &request)
	if err != nil {
		return "", []string{}, err
	}

	queryResult := response.GetQueryResult()
	fulfillmentText := queryResult.GetFulfillmentText()
	fulfillmentMessages := queryResult.FulfillmentMessages[0].GetText().Text
	return fulfillmentText, fulfillmentMessages, nil
}

//DetectIntentAudio : Detect Intent via audio and get text message from dialogflow
func DetectIntentAudio(projectID, sessionID, languageCode string, audioBytes []byte) (string, []string, error) {
	ctx := context.Background()

	sessionClient, err := dialogflow.NewSessionsClient(ctx)
	if err != nil {
		return "", []string{}, err
	}
	defer sessionClient.Close()

	if projectID == "" || sessionID == "" {
		return "", []string{}, fmt.Errorf("Received empty project (%s) or session (%s)", projectID, sessionID)
	}

	sessionPath := fmt.Sprintf("projects/%s/agent/sessions/%s", projectID, sessionID)

	// In this example, we hard code the encoding and sample rate for simplicity.
	audioConfig := dialogflowpb.InputAudioConfig{AudioEncoding: dialogflowpb.AudioEncoding_AUDIO_ENCODING_OGG_OPUS, SampleRateHertz: 16000, LanguageCode: languageCode}

	queryAudioInput := dialogflowpb.QueryInput_AudioConfig{AudioConfig: &audioConfig}

	// audioBytes, err := ioutil.ReadFile(audioFile)
	// if err != nil {
	// 	return "", err
	// }

	queryInput := dialogflowpb.QueryInput{Input: &queryAudioInput}
	request := dialogflowpb.DetectIntentRequest{Session: sessionPath, QueryInput: &queryInput, InputAudio: audioBytes}

	response, err := sessionClient.DetectIntent(ctx, &request)
	if err != nil {
		return "", []string{}, err
	}

	queryResult := response.GetQueryResult()
	fmt.Printf("Query : %+v\n", queryResult.QueryText)
	fulfillmentText := queryResult.GetFulfillmentText()
	fulfillmentMessages := queryResult.FulfillmentMessages[0].GetText().Text
	return fulfillmentText, fulfillmentMessages, nil
}

func handleBot(c *gin.Context) {
	botSession := BotSession{}
	if err := c.ShouldBindJSON(&botSession); err != nil {
		logrus.Println(err)
		c.AbortWithError(http.StatusBadRequest, err)
	}
	sessionID := botSession.SessionID
	fmt.Println(botSession.Request[:10])
	var (
		fulfillmentText string
		fulfillmentMessages []string
		err    error
	)
	switch botSession.RequestType {
	case "Text":
		text := botSession.Request
		fulfillmentText, fulfillmentMessages, err = DetectIntentText(projectID, sessionID, languageCode, string(text))
	case "Audio":
		audio := []byte(botSession.Request)
		fulfillmentText, fulfillmentMessages, err = DetectIntentAudio(projectID, sessionID, languageCode, audio)
	}
	if err != nil {
		logrus.Println(err)
		c.AbortWithError(http.StatusBadRequest, err)
	}
	c.JSON(http.StatusOK, gin.H{
		"sessionID":    sessionID,
		"languageCode": languageCode,
		"response": gin.H{
			"text": fulfillmentText,
			"messages": fulfillmentMessages,
		},
	})
}
//BotSession is data change between bot and api
type BotSession struct {
	SessionID   string	`json:"sessionID"`
	RequestType string	`json:"requestType"`
	Request     []byte	`json:"request"`
}
