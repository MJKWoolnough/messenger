package main

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

func GetGender(gender string) Gender {
	switch gender {
	case "MALE":
		return GenderMale
	case "FEMALE":
		return GenderFemale
	default:
		return GenderNeutral
	}
}

type User struct {
	ID                        string
	Name, ShortName, Username string
	Gender                    Gender
	Nickname                  string
}
