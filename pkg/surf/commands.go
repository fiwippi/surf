package surf

import (
	"fmt"
	"os"
	"reflect"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/utils/json/option"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"surf/internal/pretty"
	"surf/pkg/voice"
)

var titleCaser = cases.Title(language.English)

var commands = []api.CreateCommandData{
	{
		Name:        "join",
		Description: "Join the voice channel you are on",
	},
	{
		Name:        "leave",
		Description: "Leave the current voice channel",
	},
	{
		Name:        "play",
		Description: "Play a track",
		Options: []discord.CommandOption{
			&discord.StringOption{
				OptionName:  "track",
				Description: "Search term or URL link to track",
				Required:    true,
			},
		},
	},
	{
		Name:        "playnext",
		Description: "Play a track or adds it to the front of the queue",
		Options: []discord.CommandOption{
			&discord.StringOption{
				OptionName:  "track",
				Description: "Search term or URL link to track",
				Required:    true,
			},
		},
	},
	{
		Name:        "skip",
		Description: "Skip the currently playing track",
	},
	{
		Name:        "pause",
		Description: "Pause the currently playing track",
	},
	{
		Name:        "resume",
		Description: "Resume the currently playing track",
	},
	{
		Name:        "seek",
		Description: "Seek to a specific time in the track",
		Options: []discord.CommandOption{
			&discord.StringOption{
				OptionName:  "time",
				Description: "Time to seek to",
				Required:    true,
			},
		},
	},
	{
		Name:        "loop",
		Description: "Loop or stops looping the current track",
	},
	{
		Name:        "queue",
		Description: "View the queue of tracks",
		Options: []discord.CommandOption{
			&discord.IntegerOption{
				OptionName:  "page",
				Description: "Which page of the queue to view (25 tracks per page)",
				Required:    false,
			},
		},
	},
	{
		Name:        "np",
		Description: "View info about the track playing",
	},
	{
		Name:        "clear",
		Description: "Clear the queue",
	},
	{
		Name:        "remove",
		Description: "Remove a track from the queue",
		Options: []discord.CommandOption{
			&discord.IntegerOption{
				OptionName:  "position",
				Description: "Queue position of track",
				Required:    true,
			},
			&discord.IntegerOption{
				OptionName:  "end",
				Description: "Remove up to and including this position",
				Required:    false,
			},
		},
	},
	{
		Name:        "move",
		Description: "Moves a track to a different position in the queue",
		Options: []discord.CommandOption{
			&discord.IntegerOption{
				OptionName:  "from",
				Description: "Queue position of track",
				Required:    true,
			},
			&discord.IntegerOption{
				OptionName:  "to",
				Description: "Queue position to move track to",
				Required:    true,
			},
		},
	},
	{
		Name:        "shuffle",
		Description: "Shuffles the queue",
	},
}

func init() {
	// This is the first piece of code that runs, so it contains the logger code
	zerolog.SetGlobalLevel(zerolog.TraceLevel)
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: "15:04:05"})

	c := reflect.ValueOf(&client{})
	log.Debug().Int("method_num", c.NumMethod()).Msg("reflection of client")

	for _, cmd := range commands {
		// We get the method for the command we want
		v := c.MethodByName(titleCaser.String(cmd.Name))
		if !v.IsValid() {
			log.Fatal().Str("command", cmd.Name).Msg("command does not exist")
		}
	}
}

// Handler for the commands

func interactionCreateEvent(c *client) interface{} {
	return func(e *gateway.InteractionCreateEvent) {
		// We only want to accept command interactions i.e. slash-commands
		ci, ok := e.Data.(*discord.CommandInteraction)
		if !ok {
			return
		}

		// For voice commands we first ensure the user is in a voice channel
		ctx, err := voice.CreateContext(c.state, e, ci)
		if err != nil {
			log.Error().Err(err).Str("user", e.Sender().Username).Msg("user is not in voice channel")
			c.textResp(voice.SessionContext{Event: e}, "You must be in a voice channel", true, false)
			return
		}

		// Now we ensure the user is in the same voice channel
		if !c.manager.SameVoiceChannel(ctx) {
			log.Error().Err(err).Str("user", e.Sender().Username).Msg("user is not in the same voice channel")
			c.textResp(voice.SessionContext{Event: e}, "You must be in the same voice channel", true, false)
			return
		}

		// We get the method for the command we want
		v := reflect.ValueOf(c).MethodByName(titleCaser.String(ci.Name))
		if !v.IsValid() {
			log.Error().Str("command", ci.Name).Msg("invalid command received")
			c.textResp(ctx, "Invalid command", true, false)
			return
		}

		// Call the command
		log.Info().Str("user", ctx.User.Username).Str("command", ci.Name).Str("args", ctx.Args()).
			Str("guild", ctx.Guild).Str("channel", ctx.Voice).Msg("interaction")
		args := []reflect.Value{reflect.ValueOf(ctx)}
		v.Call(args)
	}
}

// Command functions

func (c *client) Join(ctx voice.SessionContext) {
	err := c.manager.JoinVoice(ctx)
	if err != nil {
		log.Error().Err(err).Msg("failed to join voice channel")
		c.textResp(ctx, "Failed...", true, false)
	} else {
		c.textResp(ctx, "Hi!", false, false)
	}
}

func (c *client) Leave(ctx voice.SessionContext) {
	err := c.manager.LeaveVoice(ctx)
	if err != nil {
		log.Error().Err(err).Msg("failed to leave voice channel")
		c.textResp(ctx, "Failed...", true, false)
	} else {
		c.textResp(ctx, "Bye!", false, false)
	}
}

func (c *client) Play(ctx voice.SessionContext) {
	c.textResp(ctx, "N/A", false, true)
	resp, err := c.manager.Play(ctx)
	if err != nil {
		log.Error().Err(err).Str("track", ctx.FirstArg()).Msg("failed to play track")
		c.editRespFailed(ctx, resp)
	} else {
		c.editResp(ctx, resp)
	}
}

func (c *client) Playnext(ctx voice.SessionContext) {
	c.textResp(ctx, "N/A", false, true)
	resp, err := c.manager.PlayNext(ctx)
	if err != nil {
		log.Error().Err(err).Str("track", ctx.FirstArg()).Msg("failed to play track next")
		c.editRespFailed(ctx, resp)
	} else {
		c.editResp(ctx, resp)
	}
}

func (c *client) Skip(ctx voice.SessionContext) {
	err := c.manager.Skip(ctx)
	if err != nil {
		log.Error().Err(err).Msg("failed to skip track")
		c.textResp(ctx, "Failed...", true, false)
	} else {
		c.textResp(ctx, "Skipped", false, false)
	}
}

func (c *client) Pause(ctx voice.SessionContext) {
	err := c.manager.Pause(ctx)
	if err != nil {
		log.Error().Err(err).Msg("failed to pause track")
		c.textResp(ctx, "Failed...", true, false)
	} else {
		c.textResp(ctx, "Paused", false, false)
	}
}

func (c *client) Resume(ctx voice.SessionContext) {
	err := c.manager.Resume(ctx)
	if err != nil {
		log.Error().Err(err).Msg("failed to resume track")
		c.textResp(ctx, "Failed...", true, false)
	} else {
		c.textResp(ctx, "Resumed", false, false)
	}
}

func (c *client) Seek(ctx voice.SessionContext) {
	c.textResp(ctx, "N/A", false, true)
	t, err := c.manager.Seek(ctx)
	if err != nil {
		log.Error().Err(err).Msg("failed to seek track")
		c.editResp(ctx, "Failed...")
	} else {
		c.editResp(ctx, fmt.Sprintf("Seek to `%s`", pretty.Duration(t)))
	}
}

func (c *client) Loop(ctx voice.SessionContext) {
	l, err := c.manager.Loop(ctx)
	if err != nil {
		log.Error().Err(err).Msg("failed to loop track")
		c.textResp(ctx, "Failed...", true, false)
	} else if l {
		c.textResp(ctx, "Looping", false, false)
	} else {
		c.textResp(ctx, "Not looping", false, false)
	}
}

func (c *client) Queue(ctx voice.SessionContext) {
	resp, err := c.manager.Queue(ctx)
	if err != nil {
		log.Error().Err(err).Msg("failed to send queue")
		c.textResp(ctx, "Failed...", true, false)
	} else {
		c.textResp(ctx, resp, false, false)
	}
}

func (c *client) Np(ctx voice.SessionContext) {
	resp, err := c.manager.NowPlaying(ctx)
	if err != nil {
		log.Error().Err(err).Msg("failed to get track now playing")
		c.textResp(ctx, "Failed...", true, false)
	} else {
		c.textResp(ctx, resp, false, false)
	}
}

func (c *client) Clear(ctx voice.SessionContext) {
	err := c.manager.Clear(ctx)
	if err != nil {
		log.Error().Err(err).Msg("failed to clear queue")
		c.textResp(ctx, "Failed...", true, false)
	} else {
		c.textResp(ctx, "Cleared", false, false)
	}
}

func (c *client) Remove(ctx voice.SessionContext) {
	msg, err := c.manager.Remove(ctx)
	if err != nil || msg == "" {
		log.Error().Err(err).Msg("failed to remove track from queue")
		c.textResp(ctx, "Failed...", true, false)
	} else {
		c.textResp(ctx, msg, false, false)
	}
}

func (c *client) Move(ctx voice.SessionContext) {
	msg, err := c.manager.Move(ctx)
	if err != nil || msg == "" {
		log.Error().Err(err).Msg("failed to move track in queue")
		c.textResp(ctx, "Failed...", true, false)
	} else {
		c.textResp(ctx, msg, false, false)
	}
}

func (c *client) Shuffle(ctx voice.SessionContext) {
	err := c.manager.Shuffle(ctx)
	if err != nil {
		log.Error().Err(err).Msg("failed to shuffle queue")
		c.textResp(ctx, "Failed...", true, false)
	} else {
		c.textResp(ctx, "Shuffled", false, false)
	}
}

// Sending responses

func (c *client) textResp(ctx voice.SessionContext, text string, hidden, deferred bool) {
	data := api.InteractionResponse{
		Type: api.MessageInteractionWithSource,
		Data: &api.InteractionResponseData{
			Content: option.NewNullableString(text),
		},
	}
	if deferred {
		data.Type = api.DeferredMessageInteractionWithSource
	}
	if hidden {
		data.Data.Flags = api.EphemeralResponse
	}

	if err := c.state.RespondInteraction(ctx.Event.ID, ctx.Event.Token, data); err != nil {
		log.Error().Err(err).Interface("id", ctx.Event.ID).Str("resp", text).Msg("failed to send text response")
		return
	}
}

func (c *client) editRespFailed(ctx voice.SessionContext, resp string) {
	if resp == "" {
		resp = "Failed..."
	}
	c.editResp(ctx, resp)
}

func (c *client) editResp(ctx voice.SessionContext, text string) {
	data := api.EditInteractionResponseData{
		Content: option.NewNullableString(text),
	}

	if _, err := c.state.EditInteractionResponse(c.self.ID, ctx.Event.Token, data); err != nil {
		log.Error().Err(err).Interface("id", ctx.Event.ID).Str("resp", text).Msg("failed to send text response")
		return
	}
}
