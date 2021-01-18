package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

// 	specify,_ := time.Parse(time.RFC3339, "2021-01-18T22:30:00+08:00")
// 	fmt.Println(specify)
// 	msg, err := getSchedule([]string{"不知火フレア", "宝鐘マリン"}, specify.In(time.FixedZone("UTC+9",9*60*60)))
// 	if err!=nil{
// 		fmt.Println(err)
// 		return
// 	}
// 	fmt.Println(msg)

type Schedule struct{
	Time time.Time	`json:"time"`
	Name string	`json:"name"`
	URL	string	`json:"youtube_url"`
}
type StreamStatus struct {
	ActualStartTime	time.Time	`json:"actualStartTime"`
	ScheduledStartTime time.Time	`json:"scheduledStartTime"`
	ActualEndTime	time.Time	`json:"actualEndTime"`
	ConcurrentViewers	int	`json:"concurrentViewers"`
}
func getSchedule(names []string, datetime ...time.Time)(string,error){
	message := ""
	now := time.Now().In(time.FixedZone("UTC+9",9*60*60))
	yyyy, mm, dd := now.Date()
	today := time.Date(yyyy, mm, dd, 0, 0, 0, 0, now.Location())
	//取得整個行程表
	fullSchedules := []Schedule{}
	for i, date:=0, today.AddDate(0, 0, -1) ; i < 3; i++{
		schedules := []Schedule{}
		resp, err := http.Get(
			fmt.Sprintf("https://holocrawler.herokuapp.com/api/HoloSchedule?month=%v&date=%v",
						int(date.Month()), date.Day()),
		)
		if err != nil {
			return "",err
		}
		bodyByte, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return "",err
		}
		json.Unmarshal(bodyByte, &schedules)
		fullSchedules = append(fullSchedules, schedules...)
		date = date.AddDate(0,0,1)
	}
	//時間範圍為UTC+9昨天0時到UTC+9明天24時
	//最接近範圍使用1 millisecond
	start := today.AddDate(0,0,-1)
	end := today.AddDate(0,0,+2).Add(time.Duration(-1)*time.Millisecond)
	//判斷是否在時間內
	flag := false
	switch len(datetime){
	//單一時間點:往前找並確認是否關台。若關台，在往後找是否之後預定。
	case 1:
		flag = getPosition(datetime[0], start, end)==1
		if flag {
			for _,name := range names{
				subSchedules := scheduleNameFilter(fullSchedules, name)
				nextSchedules := []Schedule{}
				subMessage := ""
				for i,v := range subSchedules{
					if v.Time.After(datetime[0]){
						subSchedules = subSchedules[:i]
						if i+1 <= len(subSchedules){
							nextSchedules = subSchedules[i+1:]
						}
						break
					}
				}
				if len(subSchedules)==0 {
					subMessage = fmt.Sprintf("[%v]目前並沒有開台喔!\n", name)
							subMessage = fmt.Sprintf("%s>>下次直播相關資訊:", subMessage)
							if len(nextSchedules)==0{
								subMessage = fmt.Sprintf("%s 無",subMessage)
							}else{
								nextSchedule := nextSchedules[0]
								nextSchedule.Time = nextSchedule.Time.In(time.FixedZone("UTC+8", 8*60*60))
								nTime := nextSchedule.Time
								subMessage = fmt.Sprintf("%s\n表定時間:%v月%v日 %v時%v分開始\n%s",
														subMessage, int(nTime.Month()), nTime.Day(), nTime.Hour(), nTime.Minute(), nextSchedule.URL)

							}
				}else{
					last := subSchedules[len(subSchedules)-1]
					status := StreamStatus{}
					resp, err := http.Get(
						fmt.Sprintf("https://holocrawler.herokuapp.com/api/HoloSchedule/status?url=%s",last.URL),
					)
					if err!=nil{
						return "",err
					}
					bodyByte, err := ioutil.ReadAll(resp.Body)
					if err != nil {
						return "",err
					}
					json.Unmarshal(bodyByte, &status)
					if !status.ActualStartTime.IsZero(){
						if status.ActualEndTime.IsZero(){
							// 還在開台中
							subMessage = fmt.Sprintf("[%v]現在正在直播中喔!目前 %v 人正在觀看:\nlink:%v\n", name, status.ConcurrentViewers, last.URL)
						}else{
							// 表示已經關台
							subMessage = fmt.Sprintf("[%v]目前已經關台了喔!\nlink:%v\n", name, last.URL)
							subMessage = fmt.Sprintf("%s>>下次直播相關資訊:", subMessage)
							if len(nextSchedules)==0{
								subMessage = fmt.Sprintf("%s 無",subMessage)
							}else{
								nextSchedule := nextSchedules[0]
								nextSchedule.Time = nextSchedule.Time.In(time.FixedZone("UTC+8", 8*60*60))
								nTime := nextSchedule.Time
								subMessage = fmt.Sprintf("%s\n表定時間:%v月%v日 %v時%v分開始\n%s",
														subMessage, int(nTime.Month()), nTime.Day(), nTime.Hour(), nTime.Minute(), nextSchedule.URL)

							}
						}
					}else{
						// 還沒開台
						// 上方已經定義
					}
				}
				message = fmt.Sprintf("%s\n%s", message, subMessage)
			}
			return message, nil
		}
	//範圍時間點:尋找有效範圍內的表定時間表
	case 2:
		startPos, endPos := getPosition(datetime[0], start, end), getPosition(datetime[1], start, end)
		flag = startPos==1&&endPos==1 || startPos!=endPos
		if flag {
			if startPos==0 && endPos==1 {
				message = fmt.Sprintf("%v月%v日前的行程我不知道，不過我知道%v月%v日到%v月%v日",
									int(start.Month()), start.Day(), 
									int(start.Month()), start.Day(),
									int(datetime[1].Month()), datetime[1].Day())
			}else if startPos==1 && endPos==2 {
				message = fmt.Sprintf("%v月%v日後的行程我不知道，不過我知道%v月%v日到%v月%v日",
									int(end.Month()), end.Day(),
									int(datetime[0].Month()), datetime[0].Day(),
									int(end.Month()), end.Day())
			}else if startPos==endPos{
				message = fmt.Sprintf("我知道%v月%v日到%v月%v日",int(datetime[0].Month()), datetime[0].Day(),int(datetime[1].Month()), datetime[1].Day())
			}else{
				message = fmt.Sprintf("%v月%v日前的行程和%v月%v日後的行程我不知道，不過我知道%v月%v日到%v月%v日",
									int(start.Month()), start.Day(), 
									int(end.Month()), end.Day(),
									int(start.Month()), start.Day(),
									int(end.Month()), end.Day())
			}
			subSchedules := scheduleNameFilter(fullSchedules, names...)
			subSchedules = scheduleDateFilter(subSchedules, datetime[0], datetime[1])
			if len(subSchedules)!=0{
				message = fmt.Sprintf("%s間 [%v] 共有%v個預定直播，分別是(台灣時間):", message, strings.Join(names, "]["), len(subSchedules))
				for _,s := range subSchedules{
					tTime := s.Time.Add(time.Duration(-1)*time.Hour)
					message = fmt.Sprintf("%s\n%v月%v日%v時%v分 %s\nlink:%s", message, int(tTime.Month()), tTime.Day(), tTime.Hour(), tTime.Minute(), s.Name, s.URL)
				}
			}else{
				message = fmt.Sprintf("%s間 [%v] 共有%v個預定直播。", message, strings.Join(names, "]["), len(subSchedules))
			}
			fmt.Println(message)
			return message,nil
		}
	//不支援多時間範圍or單點搜尋
	default:
		return "",fmt.Errorf("reach max time range")
	}
	//請求不合法時間範圍
	if !flag {
		message = "ぺこら機器人只知道日本時間的昨天、今天、明天的時程。你給的時間太%s了，pekora不知道"
		if datetime[0].Before(start){
			message = fmt.Sprintf(message, "早")
		}else{
			message = fmt.Sprintf(message, "晚")
		}
		fmt.Println(message)
	}
	return message, nil
}

func getPosition(t, start, end time.Time)int{
	if t.Before(end.Add(time.Duration(-1)*time.Millisecond)) && t.After(start.Add(time.Duration(1)*time.Millisecond)){
		//中間
		return 1
	}else if t.Before(end.Add(time.Duration(-1)*time.Millisecond)) && t.Before(start.Add(time.Duration(1)*time.Millisecond)){
		//左邊
		return 0
	}
	//右邊
	return 2
}
func scheduleNameFilter(schedules []Schedule, names ...string) (ret []Schedule){
	checkTable := map[string]bool{}
	for _,name := range names{
		checkTable[name] = true
	}
	for _,s := range schedules{
		if _, ok := checkTable[s.Name]; ok {
			ret = append(ret, s)
		}
	}
	return
}
func scheduleDateFilter(schedules []Schedule, start, end time.Time) (ret []Schedule){
	for _,s := range schedules{
		if getPosition(s.Time, start, end)==1 {
			ret = append(ret, s)
		}
	}
	return
}