package service

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"time"
	"x-ui/database"
	"x-ui/database/model"

	"errors"

	"github.com/xtls/xray-core/common/uuid"
	"gorm.io/gorm"
)

type SubscriptionService struct {
	inboundService InboundService
	settingService SettingService
}

func (s *SubscriptionService) GetSubscriptionByToken(token string) (*model.Subscription, error) {
	db := database.GetDB()
	subscription := &model.Subscription{}
	err := db.Where("token = ?", token).First(&subscription).Error
	if err != nil {
		return nil, err
	}
	return subscription, nil
}

func (s *SubscriptionService) GetSubscriptions(userId int) ([]*model.Subscription, error) {
	db := database.GetDB()
	var subscriptions []*model.Subscription
	err := db.Model(model.Subscription{}).Find(&subscriptions).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	return subscriptions, nil
}

func (s *SubscriptionService) getAvailablePorts(c int) []int {
	seed := time.Now().UnixNano()
	myRand := rand.New(rand.NewSource(seed))
	ports := make([]int, 0)
	allSettings, _ := s.settingService.GetAllSetting()
	from := allSettings.ConfigPortStart
	if from == 0 {
		from = 10000
	}
	to := allSettings.ConfigPortEnd
	if to == 0 {
		to = 60000
	}
	gap := to - from
	fmt.Printf("from: %d, to: %d, gap: %d\n", from, to, gap)
	i := 0
	for {
		port := myRand.Intn(gap) + from
		e, _ := s.inboundService.CheckPortExist(port)
		if !e {
			ports = append(ports, port)
			i++
			if i == c {
				return ports
			}
		}
	}
}
func (s *SubscriptionService) CreateAndSaveInbounds(tx *gorm.DB, subscription *model.Subscription) ([]int, error) {
	sampleId := subscription.SampleId

	availableCount := subscription.AvailableCount

	inbound, _ := s.inboundService.GetInbound(sampleId)
	if inbound == nil {
		return nil, errors.New("sampleId无效")
	}

	inboundIds := make([]int, 0)
	ports := s.getAvailablePorts(availableCount)
	fmt.Printf("ports:%v\n", ports)
	for i := 0; i < availableCount; i++ {
		newInbound := &model.Inbound{
			Port:       ports[i],
			Enable:     subscription.Enable,
			Protocol:   inbound.Protocol,
			Sniffing:   inbound.Sniffing,
			Remark:     fmt.Sprintf("订阅节点，请勿删除！copy-%d", sampleId),
			UserId:     subscription.UserId,
			ExpiryTime: subscription.ExpiryTime,
			Tag:        fmt.Sprintf("subscription-%v", ports[i]),
		}
		// 处理 settings
		settings := inbound.Settings
		var j map[string]interface{}
		json.Unmarshal([]byte(settings), &j)
		uid := uuid.New()
		j["clients"].([]interface{})[0].(map[string]interface{})["id"] = uid.String()
		modifiedSettings, _ := json.Marshal(j)
		newInbound.Settings = string(modifiedSettings)

		// 处理 streamSettings
		streamSettings := inbound.StreamSettings
		var ssJson map[string]interface{}
		err := json.Unmarshal([]byte(streamSettings), &ssJson)
		if err != nil {
			fmt.Printf("json.Unmarshal failed: %v\n", err)
			return nil, err
		}
		for k := range ssJson {
			// http settings
			if k == "httpSettings" {
				httpSettings := ssJson["httpSettings"].(map[string]interface{})
				if httpSettings != nil {
					httpSettings["path"] = fmt.Sprintf("/auto-http-%v", ports[i])
					ssJson["httpSettings"] = httpSettings
				}
			} else if k == "wsSettings" {
				// ws settings
				wsSettings := ssJson["wsSettings"].(map[string]interface{})
				if wsSettings != nil {
					wsSettings["path"] = fmt.Sprintf("/auto-ws-%v", ports[i])
					ssJson["wsSettings"] = wsSettings
				}
			}
		}

		modifiedStreamSettings, _ := json.Marshal(ssJson)
		newInbound.StreamSettings = string(modifiedStreamSettings)

		err = tx.Save(&newInbound).Error
		if err != nil {
			return nil, err
		}
		inboundIds = append(inboundIds, newInbound.Id)
	}
	// inboundIds, err := s.inboundService.AddSubInbounds(newInbounds)
	// if err != nil {
	// 	return nil, err
	// }
	return inboundIds, nil
}
func (s *SubscriptionService) AutoUpdateSubscription(subscription *model.Subscription) ([]int, error) {
	db := database.GetDB()
	tx := db.Begin()
	var err error
	defer func() {
		if err == nil {
			tx.Commit()
		} else {
			tx.Rollback()
		}
	}()
	var ids []int
	err = json.Unmarshal([]byte(subscription.InboundIds), &ids)
	if err != nil {
		fmt.Printf("json.Unmarshal error: %v", err)
		return nil, errors.New("json.Unmarshal error")
	}
	s.inboundService.InnerDelInboundsByIds(tx, ids)

	inboundIds, err := s.CreateAndSaveInbounds(tx, subscription)
	if err != nil {
		return nil, errors.New("创建新的节点 error")
	}
	idsStr, err := json.Marshal(inboundIds)
	if err != nil {
		return nil, errors.New("创建新的节点 error")
	}
	subscription.InboundIds = string(idsStr)
	// 更新订阅
	err = tx.Save(subscription).Error
	return inboundIds, nil
}
func (s *SubscriptionService) AddSubscription(subscription *model.Subscription) error {

	availableCount := subscription.AvailableCount
	if availableCount == 0 || availableCount > 10 {
		return errors.New("数量必须大于0,小于10")
	}

	db := database.GetDB()
	tx := db.Begin()
	var err error
	defer func() {
		if err == nil {
			tx.Commit()
		} else {
			tx.Rollback()
		}
	}()

	inboundIds, err := s.CreateAndSaveInbounds(tx, subscription)
	if err != nil {
		return errors.New("添加节点失败")
	}

	subscription.InboundIds = strings.Join(strings.Fields(fmt.Sprint(inboundIds)), ",")

	return tx.Save(subscription).Error
}

func (s *SubscriptionService) DelSubscription(id int) error {
	db := database.GetDB()
	tx := db.Begin()
	var err error
	defer func() {
		if err == nil {
			tx.Commit()
		} else {
			tx.Rollback()
		}
	}()
	subscription := &model.Subscription{}
	err = tx.First(subscription, id).Error
	if err != nil {
		fmt.Printf("find subscription error: %v \n", err)
		return err
	}
	var inboundIds []int
	err = json.Unmarshal([]byte(subscription.InboundIds), &inboundIds)
	if err != nil {
		fmt.Printf("Unmarshal InboundIds error: %v \n", err)
		return err
	}
	err = s.inboundService.InnerDelInboundsByIds(tx, inboundIds)
	if err != nil {
		fmt.Printf("InnerDelInboundsByIds error: %v \n", err)
		return err
	}
	err = tx.Delete(model.Subscription{}, id).Error
	return err
}

func (s *SubscriptionService) GetSubscription(id int) (*model.Subscription, error) {
	db := database.GetDB()
	subscription := &model.Subscription{}
	err := db.Model(model.Subscription{}).First(subscription, id).Error
	if err != nil {
		return nil, err
	}
	return subscription, nil
}

func (s *SubscriptionService) UpdateSubscription(subscription *model.Subscription) error {

	var err error

	oldSubscription, err := s.GetSubscription(subscription.Id)
	if err != nil {
		return err
	}
	db := database.GetDB()
	tx := db.Begin()
	defer func() {
		if err == nil {
			tx.Commit()
		} else {
			tx.Rollback()
		}
	}()
	// oldSubscription.SampleId = subscription.SampleId
	// oldSubscription.AvailableCount = subscription.AvailableCount
	oldSubscription.AutoUpdate = subscription.AutoUpdate
	oldSubscription.Remark = subscription.Remark
	oldSubscription.Enable = subscription.Enable
	oldSubscription.ExpiryTime = subscription.ExpiryTime

	var inboundIds []int
	err = json.Unmarshal([]byte(subscription.InboundIds), &inboundIds)
	if err != nil {
		fmt.Printf("Unmarshal InboundIds error: %v \n", err)
		return err
	}
	err = s.inboundService.UpdInboundsStatusByIds(tx, inboundIds, subscription.Enable, subscription.ExpiryTime)
	if err != nil {
		fmt.Printf("UpdInboundsStatus error: %v \n", err)
		return err
	}
	err = tx.Save(oldSubscription).Error

	return err
}
