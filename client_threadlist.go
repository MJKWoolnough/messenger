package messenger

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"time"

	"vimagination.zapto.org/errors"
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

type apiError struct {
	Code         int    `json:"code"`
	APIErrorCode int    `json:"api_error_code"`
	Summary      string `json:"summary"`
	Description  string `json:"description"`
	DebugInfo    string `json:"debug_info"`
}

func (a apiError) Error() string {
	return a.Description
}

type threadList struct {
	List struct {
		Data struct {
			Viewer struct {
				MessageThreads struct {
					Nodes []struct {
						ThreadKey struct {
							ThreadFBID  string `json:"thread_fbid"`
							OtherUserID string `json:"other_user_id"`
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
	Error apiError `json:"error"`
}

type Thread struct {
	ID                        string
	Name                      string
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

func (c *Client) UpdateThreadList() error {
	post := make(url.Values)
	post.Set("batch_name", "MessengerGraphQLThreadlistFetcher")
	post.Set("queries", fmt.Sprintf("{\"o0\":{\"doc_id\":%q,\"query_params\":{\"limit\":99,\"before\":null,\"tags\":[],\"isWorkUser\":0,\"includeDeliveryReceipts\":true,\"includeSeqID\":false}}}", c.docIDs["MessengerGraphQLThreadlistFetcher"]))

	resp, err := c.postForm(cAPIURL, post)
	if err != nil {
		return errors.WithContext("error getting thread list: ", err)
	}
	var list threadList
	err = json.NewDecoder(resp.Body).Decode(&list)
	resp.Body.Close()
	if err != nil {
		return errors.WithContext("error decoding thread list: ", err)
	}
	if list.Error.APIErrorCode != 0 {
		return list.Error
	}
	return c.parseThreadData(list)
}

func (c *Client) parseThreadData(list threadList) error {
	c.dataMu.Lock()
	for _, node := range list.List.Data.Viewer.MessageThreads.Nodes {
		thread := Thread{
			ID:                       node.ThreadKey.ThreadFBID,
			Name:                     node.Name,
			Type:                     getThreadType(node.ThreadType),
			Participants:             make([]string, 0, len(node.Participants.Nodes)),
			ParticipantCustomisation: make(map[string]string, len(node.Customisation.Participants)),
			UnreadCount:              node.UnreadCount,
			MessageCount:             node.MessagesCount,
			Updated:                  unixToTime(node.UpdatedTime),
		}
		if len(node.LastMessage.Nodes) > 0 {
			lm := node.LastMessage.Nodes[0]
			thread.LastMessage.Sender = lm.MessageSender.MessagingActor.ID
			thread.LastMessage.Snippet = lm.Snippet
			thread.LastMessage.Time = unixToTime(lm.Timestamp)
		}
		for _, user := range node.Participants.Nodes {
			c.setUser(User{
				ID:        user.MessagingActor.ID,
				Name:      user.MessagingActor.Name,
				ShortName: user.MessagingActor.ShortName,
				Username:  user.MessagingActor.Username,
				Gender:    getGender(user.MessagingActor.Gender),
			})
			thread.Participants = append(thread.Participants, user.MessagingActor.ID)
		}
		if thread.Type == ThreadOneToOne {
			thread.ID = node.ThreadKey.OtherUserID
			thread.Name = c.users[node.ThreadKey.OtherUserID].Name
		}
		for _, cparts := range node.Customisation.Participants {
			thread.ParticipantCustomisation[cparts.ID] = cparts.Nickname
		}
		c.threads[thread.ID] = thread
	}
	c.dataMu.Unlock()
	return nil
}

type messages struct {
	List struct {
		Data struct {
			MessageThread struct {
				UnreadCount  int    `json:"unread_count"`
				MessageCount int    `json:"message_count"`
				UpdatedTime  string `json:"updated_time_precise"`
				Messages     struct {
					PageInfo struct {
						HasPreviousPage bool `json:"has_previous_page"`
					} `json:"page_info"`
					Nodes []struct {
						TypeName string `json:"__typename"`
						Sender   struct {
							ID    string `json:"id"`
							Email string `json:"email"`
						} `json:"message_sender"`
						Timestamp string `json:"timestamp_precise"`
						Unread    bool   `json:"unread"`
						Message   struct {
							Text string `json:"text"`
						} `json:"message"`
						EMAdminText struct {
							TypeName    string `json:"__typename"`       // ADD_CONTACT, ACCEPT_PENDING_THREAD
							AddedID     string `json:"contact_added_id"` // Message request of...
							AdderID     string `json:"contact_adder_id"` // Message request by...
							AccepterID  string `json:"accepter_id"`      // Message accepted by...
							RequesterID string `json:"requester_id"`     // Message accepted of...
						} `json:"extensible_message_admin_text"`
						EMAdminTextType string `json:"extensible_message_admin_text_type"`
						Snippet         string `json:"snippet"`
					} `json:"nodes"`
				} `json:"messages"`
			} `json:"message_thread"`
		} `json:"data"`
	} `json:"o0"`
	Error apiError `json:"error"`
}

type Message struct {
	Message, Sender string
	Time            time.Time
}

type Messages []Message

func (m Messages) Len() int {
	return len(m)
}

func (m Messages) Less(i, j int) bool {
	return m[i].Time.Before(m[j].Time)
}

func (m Messages) Swap(i, j int) {
	m[i], m[j] = m[j], m[i]
}

func (c *Client) GetThread(id string) (Messages, error) {
	post := make(url.Values)
	post.Set("batch_name", "MessengerGraphQLThreadFetcher")
	post.Set("queries", fmt.Sprintf("{\"o0\":{\"doc_id\":%q,\"query_params\":{\"id\":%q,\"message_limit\":20,\"load_messages\":1,\"load_read_receipts\":false,\"before\":null}}}", c.docIDs["MessengerGraphQLThreadFetcher"], id))
	resp, err := c.postForm(cAPIURL, post)
	if err != nil {
		return nil, errors.WithContext("error getting thread messages: ", err)
	}
	var list messages
	err = json.NewDecoder(resp.Body).Decode(&list)
	if err != nil {
		return nil, errors.WithContext("error decoding thread message list: ", err)
	}
	if list.Error.APIErrorCode != 0 {
		return nil, list.Error
	}
	ms := make(Messages, 0, len(list.List.Data.MessageThread.Messages.Nodes))
	for _, node := range list.List.Data.MessageThread.Messages.Nodes {
		ms = append(ms, Message{
			Message: node.Message.Text,
			Sender:  node.Sender.ID,
			Time:    unixToTime(node.Timestamp),
		})
	}
	sort.Sort(ms)
	return ms, nil
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
