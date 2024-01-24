package model

import (
	"fmt"
	"strings"

	"golang.org/x/exp/slog"
)

func TraqMessageProcessor(messageList MessageList) (SendList, error) {
	wordsList := []WordsItem{}
	err := db.Select(&wordsList, `
		SELECT 
			word, bot_notification, me_notification, trap_id 
		FROM
			words`)
	if err != nil {
		slog.Info("Error selecting words: %v", err)
		return nil, err
	}

	usersItem := []UsersItem{}
	err = db.Select(&usersItem, `
		SELECT 
			traq_uuid, trap_id, is_bot 
		FROM
			users`)
	if err != nil {
		slog.Info("Error selecting users: %v", err)
		return nil, err
	}

	traqUuidToTrapId := make(map[string]UsersItem)
	trapIdToTraqUuid := make(map[string]UsersItem)

	for _, item := range usersItem {
		trapIdToTraqUuid[item.TrapID] = item
		traqUuidToTrapId[item.TraqUUID] = item
	}

	var sendList SendList
	// TODO: Sotatsu リファクタリングと確認頼んだ！
	for _, message := range messageList {
		var messageOwnerTrapId string
		messageOwner, ok := traqUuidToTrapId[message.TraqUuid]
		if ok {
			messageOwnerTrapId = messageOwner.TrapID
		}

		lowerContent := strings.ToLower(message.Content)
		for _, wordsItem := range wordsList {
			notifyTarget, ok := trapIdToTraqUuid[wordsItem.TrapId]
			if !ok {
				continue
			}
			if strings.Contains(lowerContent, strings.ToLower(wordsItem.Word)) {
				if !wordsItem.IncludeMe {
					if messageOwnerTrapId == notifyTarget.TrapID {
						continue
					}
				}

				if !wordsItem.IncludeBot {
					if messageOwner.IsBot {
						continue
					}
				}
				// 通知する内容を追加
				sendList = append(sendList, &Send{
					// wordがワードを登録しているUserの情報
					// messageが投稿されたワードの情報
					Word:                 wordsItem.Word,
					NotifyTargetTrapId:   notifyTarget.TrapID,
					NotifyTargetTraqUuid: notifyTarget.TraqUUID,
					MessageId:            message.Id,
					IsBot:                messageOwner.IsBot,
				})
			}
		}
	}
	// slog.Info("sendList: %v", sendList)
	return sendList, nil
}

func FindMatchingWords(messageList MessageList) ([]*NotifyInfo, error) {
	notifyInfoList := make([]*NotifyInfo, 0)

	// メッセージごとに通知対象を検索する
	for _, messageItem := range messageList {
		// メッセージに含まれている登録単語で、通知条件が合致するものを登録者別にまとめる
		matchedWordsList := make([]*MatchedWords, 0)
		err := db.Select(&matchedWordsList, `
			SELECT
				group_concat(words.word SEPARATOR '\n') AS contacted_words,
				words.trap_id AS trap_id,
				users.traq_uuid AS traq_uuid
			FROM words
			JOIN users ON words.trap_id = users.trap_id
				WHERE (
				    	((word NOT LIKE '/%/') 
				    		AND (? LIKE concat('%', word, '%')))
            			OR ((word LIKE '/%/') 
            				AND (BINARY ? REGEXP trim(BOTH '/' FROM word)))
				    )
				AND (me_notification OR
					 users.traq_uuid != ?)
				AND (bot_notification OR
					 (SELECT is_bot FROM users WHERE traq_uuid = ? LIMIT 1) = FALSE)
			GROUP BY words.trap_id`,
			messageItem.Content, messageItem.Content, messageItem.TraqUuid, messageItem.TraqUuid)
		if err != nil {
			slog.Error(fmt.Sprintf("failed to search words with message: `%s`", messageItem.Id))
			return nil, err
		}

		for _, matchedWords := range matchedWordsList {
			notifyInfo := &NotifyInfo{
				Words:                strings.Split(matchedWords.ContactedWords, "\n"),
				NotifyTargetTrapId:   matchedWords.TrapID,
				NotifyTargetTraqUuid: matchedWords.TraqUUID,
				MessageId:            messageItem.Id,
			}

			notifyInfoList = append(notifyInfoList, notifyInfo)
		}
	}

	return notifyInfoList, nil
}

type MessageItem struct {
	// メッセージUUID
	Id string `json:"id"`
	// 投稿者UUID
	TraqUuid string `json:"userId"`
	// メッセージ本文
	Content string `json:"content"`
}

type MessageList []MessageItem

type WordsItem struct {
	IncludeBot bool   `db:"bot_notification"`
	IncludeMe  bool   `db:"me_notification"`
	TrapId     string `db:"trap_id"`
	Word       string `db:"word"`
}

type UsersItem struct {
	TrapID   string `db:"trap_id"`
	TraqUUID string `db:"traq_uuid"`
	IsBot    bool   `db:"is_bot"`
}

type Send struct {
	// 含んでいた単語
	Word string
	// 送信先のuser
	NotifyTargetTrapId string
	// 送信先のuserUUID
	NotifyTargetTraqUuid string
	// 送信するメッセージのID
	MessageId string
	// BOTかどうか
	IsBot bool
}

type SendList []*Send

type NotifyInfo struct {
	Words []string
	// 送信先のuser
	NotifyTargetTrapId string
	// 送信先のuserUUID
	NotifyTargetTraqUuid string
	// 送信するメッセージのID
	MessageId string
}

type MatchedWords struct {
	ContactedWords string `db:"contacted_words"`
	TrapID         string `db:"trap_id"`
	TraqUUID       string `db:"traq_uuid"`
}
