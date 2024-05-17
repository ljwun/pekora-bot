package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	SCHEDULE_API_HOST        = "https://holocrawler.herokuapp.com/"
	SCHEDULE_API_GET_PATH    = "/api/HoloSchedule"
	SCHEDULE_API_STATUS_PATH = "/api/HoloSchedule/status"
)

type Schedule struct {
	Time time.Time `json:"time"`
	Name string    `json:"name"`
	URL  string    `json:"youtube_url"`
}
type StreamStatus struct {
	ActualStartTime    time.Time `json:"actualStartTime"`
	ScheduledStartTime time.Time `json:"scheduledStartTime"`
	ActualEndTime      time.Time `json:"actualEndTime"`
	ConcurrentViewers  int       `json:"concurrentViewers"`
}

func (ss StreamStatus) IsStreamBegun() bool {
	return !ss.ActualStartTime.IsZero()
}

func (ss StreamStatus) IsStreamClosed() bool {
	return ss.ActualEndTime.IsZero()
}

func getSchedule(memberNames []string, datetime ...time.Time) (string, error) {
	for i := range datetime {
		datetime[i] = datetime[i].In(time.FixedZone("UTC+9", 9*60*60))
		fmt.Println(datetime[i])
	}
	now := time.Now().In(time.FixedZone("UTC+9", 9*60*60))
	yyyy, mm, dd := now.Date()
	today := time.Date(yyyy, mm, dd, 0, 0, 0, 0, now.Location())
	fullSchedules, err := getFullSchedule(today)
	if err != nil {
		return "", fmt.Errorf("fail to ger full schedule, err=%w", err)
	}
	//時間範圍為UTC+9昨天0時到UTC+9明天24時
	//最接近範圍使用1 millisecond
	start := today.AddDate(0, 0, -1)
	end := today.AddDate(0, 0, +2).Add(time.Duration(-1) * time.Millisecond)
	if len(datetime) == 1 {
		return processSpecifiedTime(fullSchedules, datetime[0], start, end, memberNames)
	} else if len(datetime) == 2 {
		return processIntervalTime(fullSchedules, datetime[0], datetime[1], start, end, memberNames), nil
	}
	return "", fmt.Errorf("reach max time range")
}

type position int

const (
	LEFT_POSITION position = iota
	MIDDLE_POSITION
	RIGHT_POSITION
)

func getPosition(t, start, end time.Time) position {
	if t.Before(start.Add(time.Duration(1) * time.Millisecond)) {
		return LEFT_POSITION
	}
	if t.Before(end.Add(time.Duration(-1) * time.Millisecond)) {
		return MIDDLE_POSITION
	}
	return RIGHT_POSITION
}

func scheduleNameFilter(schedules []Schedule, names ...string) (ret []Schedule) {
	checkTable := map[string]bool{}
	for _, name := range names {
		checkTable[name] = true
	}
	for _, s := range schedules {
		if _, ok := checkTable[s.Name]; ok {
			ret = append(ret, s)
		}
	}
	return
}
func scheduleDateFilter(schedules []Schedule, start, end time.Time) (ret []Schedule) {
	for _, s := range schedules {
		if getPosition(s.Time, start, end) == 1 {
			ret = append(ret, s)
		}
	}
	return
}

func processSpecifiedTime(schedules []Schedule, specifiedTime, start, end time.Time, members []string) (string, error) {
	messagePattern := "ぺこら機器人只知道日本時間的昨天、今天、明天的時程。你給的時間太%s了，pekora不知道"
	if pos := getPosition(specifiedTime, start, end); pos == LEFT_POSITION {
		return fmt.Sprintf(messagePattern, "早"), nil
	} else if pos == RIGHT_POSITION {
		return fmt.Sprintf(messagePattern, "晚"), nil
	}
	messages := make([]string, 0, len(members))
	for _, member := range members {
		buf := make([]string, 0, 10)
		specifiedMemberSchedules := scheduleNameFilter(schedules, member)
		var precedingSchedules, followingSchedules []Schedule
		for idx, schedule := range specifiedMemberSchedules {
			if schedule.Time.Before(specifiedTime) {
				continue
			}
			precedingSchedules = specifiedMemberSchedules[:idx]
			followingSchedules = specifiedMemberSchedules[idx+1:]
			break
		}
		var targetSchedule *Schedule
		if len(precedingSchedules) == 0 {
			buf = append(buf, fmt.Sprintf("[%v]這個時間沒有開台喔!\n", member))
			buf = append(buf, ">>下次表定的下次直播資料:\n")
			if len(followingSchedules) > 0 {
				targetSchedule = &followingSchedules[0]
			} else {
				buf = append(buf, "無")
			}
		} else {
			buf = append(buf, fmt.Sprintf("[%v]這個時間有開台喔!\n", member))
			targetSchedule = &precedingSchedules[len(precedingSchedules)-1]
		}
		if targetSchedule == nil {
			messages = append(messages, strings.Join(buf, ""))
			continue
		}
		streamStatus, err := getStreamStatus(targetSchedule.URL)
		if err != nil {
			return "", fmt.Errorf("fail to get stream status, err=%w", err)
		}
		buf = append(buf, fmt.Sprintf("表定開始時間為%s\n", streamStatus.ScheduledStartTime.In(time.FixedZone("UTC+8", 8*60*60)).Format("1月2日 15點04分")))
		if streamStatus.IsStreamClosed() {
			buf = append(buf, fmt.Sprintf("不過已於%s結束直播了\n",
				streamStatus.ActualEndTime.In(time.FixedZone("UTC+8", 8*60*60)).Format("1月2日 15點04分"),
			))
		} else if streamStatus.IsStreamBegun() {
			buf = append(buf, fmt.Sprintf("直播已於%s開始了喔，目前有%d人正在觀看，快點加入\n",
				streamStatus.ActualStartTime.In(time.FixedZone("UTC+8", 8*60*60)).Format("1月2日 15點04分"),
				streamStatus.ConcurrentViewers,
			))
		} else {
			buf = append(buf, "直播還沒開始喔\n")
		}
		buf = append(buf, targetSchedule.URL)
		messages = append(messages, strings.Join(buf, ""))
	}
	return strings.Join(messages, "\n"), nil
}

func processIntervalTime(schedules []Schedule, start, end, startBoundary, endBoundary time.Time, members []string) string {
	startPos, endPos := getPosition(start, startBoundary, endBoundary), getPosition(end, startBoundary, endBoundary)
	if endPos == LEFT_POSITION {
		return fmt.Sprintf("我不知道%s前的直播", startBoundary.In(time.FixedZone("UTC+8", 8*60*60)).Format("1月2日 15點04分"))
	}
	if startPos == RIGHT_POSITION {
		return fmt.Sprintf("我不知道%s後的直播", endBoundary.In(time.FixedZone("UTC+8", 8*60*60)).Format("1月2日 15點04分"))
	}
	messages := make([]string, 0, 10)
	if startPos == endPos {
		messages = append(messages, fmt.Sprintf("我知道%s到%s的直播\n",
			start.In(time.FixedZone("UTC+8", 8*60*60)).Format("1月2日 15點04分"),
			end.In(time.FixedZone("UTC+8", 8*60*60)).Format("1月2日 15點04分"),
		))
	} else if endPos == MIDDLE_POSITION {
		messages = append(messages, fmt.Sprintf("%s到%s的直播我不知道，我只知道%s後的直播\n",
			start.In(time.FixedZone("UTC+8", 8*60*60)).Format("1月2日 15點04分"),
			startBoundary.In(time.FixedZone("UTC+8", 8*60*60)).Format("1月2日 15點04分"),
			startBoundary.In(time.FixedZone("UTC+8", 8*60*60)).Format("1月2日 15點04分"),
		))
	} else if startPos == MIDDLE_POSITION {
		messages = append(messages, fmt.Sprintf("%s到%s的直播我不知道，我只知道%s前的直播\n",
			endBoundary.In(time.FixedZone("UTC+8", 8*60*60)).Format("1月2日 15點04分"),
			end.In(time.FixedZone("UTC+8", 8*60*60)).Format("1月2日 15點04分"),
			endBoundary.In(time.FixedZone("UTC+8", 8*60*60)).Format("1月2日 15點04分"),
		))
	} else {
		messages = append(messages, fmt.Sprintf("%s到%s和%s到%s的直播我不知道，我只知道%s到%s的直播\n",
			start.In(time.FixedZone("UTC+8", 8*60*60)).Format("1月2日 15點04分"),
			startBoundary.In(time.FixedZone("UTC+8", 8*60*60)).Format("1月2日 15點04分"),
			endBoundary.In(time.FixedZone("UTC+8", 8*60*60)).Format("1月2日 15點04分"),
			end.In(time.FixedZone("UTC+8", 8*60*60)).Format("1月2日 15點04分"),
			startBoundary.In(time.FixedZone("UTC+8", 8*60*60)).Format("1月2日 15點04分"),
			endBoundary.In(time.FixedZone("UTC+8", 8*60*60)).Format("1月2日 15點04分"),
		))
	}
	targetSchedules := scheduleNameFilter(schedules, members...)
	targetSchedules = scheduleDateFilter(targetSchedules, start, end)
	messages = append(messages, fmt.Sprintf("表定共有%v個直播", len(targetSchedules)))
	if len(targetSchedules) > 0 {
		messages = append(messages, "，依序是:\n")
	}
	for _, targetSchedule := range targetSchedules {
		messages = append(messages, fmt.Sprintf("%s的[%s]\nlink:%s\n",
			targetSchedule.Time.In(time.FixedZone("UTC+8", 8*60*60)).Format("1月2日 15點04分"),
			targetSchedule.Name,
			targetSchedule.URL,
		))
	}
	return strings.Join(messages, "")
}

func getFullSchedule(currentDate time.Time) ([]Schedule, error) {
	apiUrl, err := url.Parse(SCHEDULE_API_HOST)
	if err != nil {
		return nil, fmt.Errorf("fail to parse api url, err=%w", err)
	}
	apiUrl.Path = SCHEDULE_API_GET_PATH
	fullSchedules := []Schedule{}
	for i, date := 0, currentDate.AddDate(0, 0, -1); i < 3; i++ {
		schedules := []Schedule{}
		apiUrl.RawQuery = url.Values{
			"month": []string{strconv.Itoa(int(date.Month()))},
			"date":  []string{strconv.Itoa(date.Day())},
		}.Encode()
		resp, err := http.Get(apiUrl.String())
		if err != nil {
			return nil, fmt.Errorf("fail to call api, err=%w", err)
		}
		bodyByte, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("fail to read response, err=%w", err)
		}
		if err := json.Unmarshal(bodyByte, &schedules); err != nil {
			return nil, fmt.Errorf("fail to parse schedule, err=%w", err)
		}
		fullSchedules = append(fullSchedules, schedules...)
		date = date.AddDate(0, 0, 1)
	}
	return fullSchedules, nil
}

func getStreamStatus(streamUrl string) (StreamStatus, error) {
	apiUrl, err := url.Parse(SCHEDULE_API_HOST)
	if err != nil {
		return StreamStatus{}, fmt.Errorf("fail to parse api url, err=%w", err)
	}
	apiUrl.Path = SCHEDULE_API_STATUS_PATH
	apiUrl.RawQuery = url.Values{"url": []string{streamUrl}}.Encode()
	status := StreamStatus{}
	resp, err := http.Get(apiUrl.String())
	if err != nil {
		return StreamStatus{}, fmt.Errorf("fail to call api, err=%w", err)
	}
	bodyByte, err := io.ReadAll(resp.Body)
	if err != nil {
		return StreamStatus{}, fmt.Errorf("fail to read response, err=%w", err)
	}
	if err = json.Unmarshal(bodyByte, &status); err != nil {
		return StreamStatus{}, fmt.Errorf("fail to parse schedule status, err=%w", err)
	}
	return status, nil
}
