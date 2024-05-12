package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang/protobuf/jsonpb"
	"github.com/sirupsen/logrus"
	"google.golang.org/genproto/googleapis/cloud/dialogflow/v2"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	parameter_id_member_name    string = "holoname"
	parameter_id_specified_time string = "date-time"
	field_id_start_date         string = "startDate"
	field_id_end_date           string = "endDate"
	field_id_start_time         string = "startTime"
	field_id_end_time           string = "endTime"
	field_id_start_date_time    string = "startDateTime"
	field_id_end_date_time      string = "endDateTime"
)

func handleWebhook(c *gin.Context) {
	wReq := dialogflow.WebhookRequest{}
	if err := jsonpb.Unmarshal(c.Request.Body, &wReq); err != nil {
		logrus.WithError(err).Error("Couldn't Unmarshal request to jsonpb")
		c.Status(http.StatusBadRequest)
		return
	}
	logrus.WithFields(
		logrus.Fields{
			"intent":    wReq.QueryResult.Intent.DisplayName,
			"parameter": wReq.QueryResult.Parameters,
			"text":      wReq.QueryResult.QueryText,
		},
	).Debug("receive request from dialogflow")
	params := wReq.QueryResult.Parameters.Fields
	var msg string
	switch wReq.QueryResult.Intent.DisplayName {
	case "開台詢問":
		memberNames := parseMember(params)
		specifiedTime, err := parseSpecifiedTime(params)
		if err != nil {
			logrus.WithError(err).Error("Couldn't parse specified time")
			c.AbortWithStatusJSON(http.StatusBadRequest, err)
			return
		}
		message, err := getSchedule(memberNames, specifiedTime...)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}
		logrus.WithField("message", message).Debug("success generate message")
		msg = message
	case "webhookDemo":
		msg = "2"
	default:
		msg = "err"
	}
	wRes := dialogflow.WebhookResponse{
		FulfillmentText: msg,
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
		c.AbortWithError(http.StatusBadRequest, err)
	}
}

func parseMember(params map[string]*structpb.Value) []string {
	result := make([]string, 0, 1)
	membersParam := params[parameter_id_member_name]
	if name := membersParam.GetStringValue(); name != "" {
		result = append(result, name)
	} else if namesValues := membersParam.GetListValue(); namesValues != nil {
		for _, nameValue := range namesValues.Values {
			if nameValue == nil {
				continue
			}
			name := nameValue.String()
			if name == "" {
				continue
			}
			result = append(result, nameValue.String())
		}
	}
	return result
}

func parseSpecifiedTime(params map[string]*structpb.Value) ([]time.Time, error) {
	datetime := params[parameter_id_specified_time].GetStructValue()
	if datetime == nil {
		return []time.Time{time.Now()}, nil
	}
	var startString, endString string = "", ""
	startString = datetime.Fields[field_id_start_date].GetStringValue()
	endString = datetime.Fields[field_id_end_date].GetStringValue()
	if startString == "" || endString == "" {
		startString = datetime.Fields[field_id_start_time].GetStringValue()
		endString = datetime.Fields[field_id_end_time].GetStringValue()
	}
	if startString == "" || endString == "" {
		startString = datetime.Fields[field_id_start_date_time].GetStringValue()
		endString = datetime.Fields[field_id_end_date_time].GetStringValue()
	}
	if startString == "" || endString == "" {
		return []time.Time{time.Now()}, nil
	}
	start, err := time.Parse(time.RFC3339, startString)
	if err != nil {
		return nil, fmt.Errorf("fail to parse %s, err=%w", startString, err)
	}
	end, err := time.Parse(time.RFC3339, endString)
	if err != nil {
		return nil, fmt.Errorf("fail to parse %s, err=%w", endString, err)
	}
	return []time.Time{start, end}, nil
}

//超出時間表的範圍	=>	PEKORA我只知道最近這三天喔
//詢問時間範圍		=>	對指定人物在時間內最接近當前時間的那一筆進行youtube data的獲取
// 						過去: 查詢是否已經關台
//						現在: 查詢是否正式開台
// 						過去: 查詢是否提早開台
//單一時間			=>	對指定人物在時間表內最接近當前時間的那一筆進行youtube data的獲取，
// 若在直播:現在在直播中喔，現在有xxx人在看，已經直播xxx分鐘了(附加網址)
// 若直播結束:直播在xxx分鐘前已經結束了，記錄檔在此(附加網址)
// 若還沒開始直播:直播還有xx分鐘就要開始了，現在有xxx人在等待(附加網址)
