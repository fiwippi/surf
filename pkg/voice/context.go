package voice

import (
	"errors"
	"strings"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/state"
)

type SessionContext struct {
	// Guild where the message was sent
	GID discord.GuildID
	// Voice channel the user is in
	Voice discord.ChannelID
	// Text channel the message was typed in
	Text discord.ChannelID
	// User who initiated the interaction
	User *discord.User
	// The interaction Event itself
	Event *gateway.InteractionCreateEvent
	// Options
	options []discord.CommandInteractionOption
}

func CreateContext(s *state.State, e *gateway.InteractionCreateEvent, ci *discord.CommandInteraction) (SessionContext, error) {
	// We use the state to get the voice channel the user is in
	// which also ensures they are in a voice channel
	vs, err := s.VoiceState(e.GuildID, e.SenderID())
	if err != nil {
		return SessionContext{}, err
	}

	return SessionContext{
		GID:     e.GuildID,
		Voice:   vs.ChannelID,
		Text:    e.ChannelID,
		User:    e.Sender(),
		Event:   e,
		options: ci.Options,
	}, nil
}

func (ctx *SessionContext) FirstArg() string {
	if len(ctx.options) < 1 {
		panic(errors.New("not enough args for first arg"))
	}
	return ctx.options[0].String()
}

func (ctx *SessionContext) SecondArg() string {
	if len(ctx.options) < 2 {
		panic(errors.New("not enough args for second arg"))
	}
	return ctx.options[1].String()
}

func (ctx *SessionContext) Args() string {
	if len(ctx.options) > 0 {
		args := make([]string, 0)
		for _, o := range ctx.options {
			args = append(args, o.String())
		}
		return strings.Join(args, ",")
	}
	return ""
}
