package messenger

type Gender byte

const (
	GenderMale   = 'M'
	GenderFemale = 'F'
	GenderNeuter = 'N'
)

func (g Gender) String() string {
	switch g {
	case GenderMale:
		return "Male"
	case GenderFemale:
		return "Female"
	default:
		return "Neuter"
	}
}

func getGender(gender string) Gender {
	switch gender {
	case "MALE":
		return GenderMale
	case "FEMALE":
		return GenderFemale
	default:
		return GenderNeuter
	}
}

type User struct {
	ID                        string
	Name, ShortName, Username string
	Gender                    Gender
}

func (c *Client) SetUser(u User) {
	c.dataMu.Lock()
	c.setUser(u)
	c.dataMu.Unlock()
}

func (c *Client) setUser(u User) {
	if _, ok := c.users[u.ID]; ok {
		return
	}
	c.users[u.ID] = u
}
