package bot

type Bot struct {
	Name string
	Hp   int
}

func NewBot(name string) Bot {
	return Bot{Name: name, Hp: 100}
}
