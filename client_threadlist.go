package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/MJKWoolnough/errors"
)

type ThreadType int

const (
	ThreadGroup ThreadType = iota
	ThreadOneToOne
)

func (t ThreadType) String() string {
	switch t {
	case ThreadGroup:
		return "Group"
	case ThreadOneToOne:
		return "One to one"
	default:
		return "Unknown"
	}
}

func getThreadType(t string) ThreadType {
	switch t {
	case "GROUP":
		return ThreadGroup
	case "ONE_TO_ONE":
		return ThreadOneToOne
	default:
		return -1
	}
}

type threadList struct {
	List struct {
		Data struct {
			Viewer struct {
				MessageThreads struct {
					Nodes []struct {
						ThreadKey struct {
							ThreadFBID string `json:"thread_fbid"`
						} `json:"thread_key"`
						Name        string `json:"name"`
						LastMessage struct {
							Nodes []struct {
								Snippet       string `json:"snippet"`
								MessageSender struct {
									MessagingActor struct {
										ID string `json:"id"`
									} `json:"messaging_actor"`
								} `json:"message_sender"`
								Timestamp string `json:"timestamp_precise"`
							} `json:"nodes"`
						} `json:"last_message"`
						UnreadCount   int    `json:"unread_count"`
						MessagesCount int    `json:"messages_count"`
						UpdatedTime   string `json:"updated_time_precise"`
						Customisation struct {
							Participants []struct {
								ID       string `json:"participant_id"`
								Nickname string `json:"nickname"`
							} `json:"participant_customizations"`
						} `json:"customization_info"`
						LastReadReceipt struct {
							Nodes []struct {
								Timestamp string `json:"timestamp_precise"`
							} `json:"nodes"`
						} `json:"last_read_receipt"`
						ThreadType   string `json:"thread_type"`
						Participants struct {
							Nodes []struct {
								MessagingActor struct {
									ID        string `json:"id"`
									Type      string `json:"__typename"`
									Name      string `json:"name"`
									Gender    string `json:"gender"`
									URL       string `json:"url"`
									ShortName string `json:"short_name"`
									Username  string `json:"username"`
								} `json:"messaging_actor"`
							} `json:"nodes"`
						} `json:"all_participants"`
						ReadReceipts struct {
							Nodes []struct {
								Watermark string `json:"watermark"`
								Action    string `json:"action"`
								Actor     struct {
									ID string `json:"id"`
								} `json:"actor"`
							} `json:"nodes"`
						} `json:"read_receipts"`
						DeliveryReceipts struct {
							Nodes []struct {
								Timestamp string `json:"timestamp_precise"`
							} `json:"nodes"`
						} `json:"delivery_receipts"`
					} `json:"nodes"`
				} `json:"message_threads"`
			} `json:"viewer"`
		} `json:"data"`
	} `json:"o0"`
}

type Thread struct {
	ID                        string
	Type                      ThreadType
	Participants              []string
	ParticipantCustomisation  map[string]string
	UnreadCount, MessageCount int
	Updated                   time.Time
	LastMessage               struct {
		Sender  string
		Snippet string
		Time    time.Time
	}
}

func (c *Client) GetList() error {
	post := make(url.Values)
	post.Set("batch_name", "MessengerGraphQLThreadlistFetcher")
	post.Set("queries", fmt.Sprintf("{\"o0\":{\"doc_id\":%q,\"query_params\":{\"limit\":99,\"before\":null,\"tags\":[],\"isWorkUser\":0,\"includeDeliveryReceipts\":true,\"includeSeqID\":false}}}", c.docIDs["MessengerGraphQLThreadlistFetcher"]))

	resp, err := c.PostForm(cAPIURL, post)
	if err != nil {
		return errors.WithContext("error getting thread list: ", err)
	}
	var list threadList
	err = json.NewDecoder(resp.Body).Decode(&list)
	resp.Body.Close()
	if err != nil {
		return errors.WithContext("error decoding thread list: ", err)
	}
	c.Threads = make([]Thread, len(list.List.Data.Viewer.MessageThreads.Nodes))
	for n, node := range list.List.Data.Viewer.MessageThreads.Nodes {
		c.Threads[n] = Thread{
			ID:                       node.ThreadKey.ThreadFBID,
			Type:                     getThreadType(node.ThreadType),
			Participants:             make([]string, 0, len(node.Participants.Nodes)),
			ParticipantCustomisation: make(map[string]string, len(node.Customisation.Participants)),
			UnreadCount:              node.UnreadCount,
			MessageCount:             node.MessagesCount,
			Updated:                  unixToTime(node.UpdatedTime),
		}
		if len(node.LastMessage.Nodes) > 0 {
			lm := node.LastMessage.Nodes[0]
			c.Threads[n].LastMessage.Sender = lm.MessageSender.MessagingActor.ID
			c.Threads[n].LastMessage.Snippet = lm.Snippet
			c.Threads[n].LastMessage.Time = unixToTime(lm.Timestamp)
		}
		for _, user := range node.Participants.Nodes {
			c.SetUser(User{
				ID:        user.MessagingActor.ID,
				Name:      user.MessagingActor.Name,
				ShortName: user.MessagingActor.ShortName,
				Username:  user.MessagingActor.Username,
				Gender:    getGender(user.MessagingActor.Gender),
			})
			c.Threads[n].Participants = append(c.Threads[n].Participants, user.MessagingActor.ID)
		}
		for _, cparts := range node.Customisation.Participants {
			c.Threads[n].ParticipantCustomisation[cparts.ID] = cparts.Nickname
		}
	}
	return nil
}

func unixToTime(str string) time.Time {
	if len(str) < 3 {
		return time.Unix(0, 0)
	}
	sec, err := strconv.ParseInt(str[:len(str)-3], 10, 64)
	if err != nil {
		return time.Unix(0, 0)
	}
	milli, err := strconv.ParseInt(str[len(str)-3:], 10, 64)
	if err != nil {
		return time.Unix(0, 0)
	}
	return time.Unix(sec, milli*1000000).In(time.Local)
}
