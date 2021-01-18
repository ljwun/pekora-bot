package main

import (
	"fmt"
	"net/http"
	"time"

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
	msg := fmt.Sprintln("Intent:",wReq.QueryResult.Intent.DisplayName)
	params := wReq.QueryResult.Parameters.Fields
	switch wReq.QueryResult.Intent.DisplayName {
	case "開台詢問":
		//成員
		members := []string{}
		if params["holoname"].GetStringValue() != ""{
			members = append(members, params["holoname"].GetStringValue())
		}else{
			for _, member := range params["holoname"].GetListValue().Values{
				members = append(members, member.GetStringValue())
			}
		}
		// msg = fmt.Sprintf("%sNames:%s\n", msg, strings.Join(members, ","))
		//時間
		
		if params["date-time"].GetStringValue() != "" || params["date_time"].GetStringValue() != "" {
			// msg = fmt.Sprintf("%sdatetime:%s",msg, params["date-time"].GetStringValue())
			if params["date-time"].GetStringValue() != ""{
				fmt.Println("now ",params["date-time"].GetStringValue())
				sTime,_ := time.Parse(time.RFC3339, params["date-time"].GetStringValue())
				message, err := getSchedule(members, sTime)
				if err!=nil{
					c.AbortWithError(http.StatusBadRequest, err)
				}
				fmt.Println(message)
				msg = message
			}else{
				fmt.Println("now ",params["date_time"].GetStringValue())
				sTime,_ := time.Parse(time.RFC3339, params["date_time"].GetStringValue())
				message, err := getSchedule(members, sTime)
				if err!=nil{
					c.AbortWithError(http.StatusBadRequest, err)
				}
				fmt.Println(message)
				msg = message
			}
		} else {
			datetime := struct {
				start string
				end   string
			}{}
			switch {
			case params["date-time"].GetStructValue().Fields["startDate"].GetStringValue() != "":
				datetime.start = params["date-time"].GetStructValue().Fields["startDate"].GetStringValue()
				datetime.end = params["date-time"].GetStructValue().Fields["endDate"].GetStringValue()
			case params["date-time"].GetStructValue().Fields["startDateTime"].GetStringValue() != "":
				datetime.start = params["date-time"].GetStructValue().Fields["startDateTime"].GetStringValue()
				datetime.end = params["date-time"].GetStructValue().Fields["endDateTime"].GetStringValue()
			default:
				datetime.start = params["date-time"].GetStructValue().Fields["startTime"].GetStringValue()
				datetime.end = params["date-time"].GetStructValue().Fields["endTime"].GetStringValue()
			}
			start,_ := time.Parse(time.RFC3339, datetime.start)
			end,_ := time.Parse(time.RFC3339, datetime.end)
			// msg = fmt.Sprintf("%sfrom:\n%v\nto:\n%v", msg, datetime.start, datetime.end)
			if start.Before(end){
				message, err := getSchedule(members, start, end)
				if err!=nil{
					c.AbortWithError(http.StatusBadRequest, err)
				}
				fmt.Println(message)
				msg = message
			}else{
				message, err := getSchedule(members, end, start)
				if err!=nil{
					c.AbortWithError(http.StatusBadRequest, err)
				}
				fmt.Println(message)
				msg = message
			}
		}
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
//超出時間表的範圍	=>	PEKORA我只知道最近這三天喔
//詢問時間範圍		=>	對指定人物在時間內最接近當前時間的那一筆進行youtube data的獲取
// 						過去: 查詢是否已經關台
//						現在: 查詢是否正式開台
// 						過去: 查詢是否提早開台
//單一時間			=>	對指定人物在時間表內最接近當前時間的那一筆進行youtube data的獲取，
							// 若在直播:現在在直播中喔，現在有xxx人在看，已經直播xxx分鐘了(附加網址)
							// 若直播結束:直播在xxx分鐘前已經結束了，記錄檔在此(附加網址)
							// 若還沒開始直播:直播還有xx分鐘就要開始了，現在有xxx人在等待(附加網址)

