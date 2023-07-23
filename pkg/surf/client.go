package surf

import (
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/state"
	"github.com/rs/zerolog/log"

	"surf/pkg/voice"
)

type client struct {
	self    *discord.Application
	state   *state.State
	manager *voice.Manager
}

func newClient(token, spotifyID, spotifySecret string) (*client, error) {
	// Setup the state
	id := gateway.DefaultIdentifier("Bot " + token)
	id.Presence = &gateway.UpdatePresenceCommand{
		Status: discord.OnlineStatus,
		Activities: []discord.Activity{{
			Name: "Surfing... 🌊🌊🌊",
			Type: discord.GameActivity,
		}},
	}

	s := state.NewWithIdentifier(id)
	c := &client{
		state: s,
	}

	// Add handlers for events
	c.state.AddHandler(interactionCreateEvent(c))

	// Ensure the overlaying discord application exists
	app, err := c.state.CurrentApplication()
	if err != nil {
		return nil, err
	}
	c.self = app

	// Create the voice manager
	m, err := voice.NewManager(s, spotifyID, spotifySecret)
	if err != nil {
		return nil, err
	}
	c.manager = m

	return c, nil
}

func (c *client) deleteOldCommands(existingCommands []discord.Command) {
	for _, oldCmd := range existingCommands {
		found := false
		for _, newCmd := range commands {
			if oldCmd.Name == newCmd.Name && len(oldCmd.Options) == len(newCmd.Options) {
				found = true
			}
		}

		if !found {
			log.Debug().Str("name", oldCmd.Name).Msg("deleting command")
			err := c.state.DeleteCommand(c.self.ID, oldCmd.ID)
			if err != nil {
				log.Fatal().Err(err).Str("cmd", oldCmd.Name).Msg("could not delete non-existent commands")
			}
		}
	}
}

func (c *client) createNewCommands(existingCommands []discord.Command) {
	for _, newCmd := range commands {
		found := false
		for _, command := range existingCommands {
			if command.Name == newCmd.Name && len(command.Options) == len(newCmd.Options) {
				found = true
			}
		}

		if !found {
			log.Debug().Str("name", newCmd.Name).Msg("creating command")
			_, err := c.state.CreateCommand(c.self.ID, newCmd)
			if err != nil {
				log.Fatal().Str("cmd", newCmd.Name).Err(err).Msg("could not register new commands")
			}
		}
	}
}
