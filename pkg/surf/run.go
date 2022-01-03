package surf

import (
	"context"
	"os"
	"os/signal"

	"surf/internal/log"
)

func Run(token string) error {
	c, err := newClient(token)
	if err != nil {
		return err
	}

	// Run the bot
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	if err := c.state.Open(ctx); err != nil {
		return err
	}
	log.Info().Msg("bot loading...")

	// Delete old commands which are no longer required and create new ones
	existingCommands, err := c.state.Commands(c.self.ID)
	if err != nil {
		log.Fatal().Err(err).Msg("could not get existing commands")
	}
	c.deleteOldCommands(existingCommands)
	c.createNewCommands(existingCommands)

	log.Info().Str("user", c.self.Name).Msg("bot ready")
	<-ctx.Done() // block until Ctrl+C

	if err := c.state.Close(); err != nil {
		return err
	}
	return nil
}
