package di

import (
	"context"

	"ethereum-fetcher/cmd"
	"ethereum-fetcher/internal/app"
	"ethereum-fetcher/internal/network"
	"ethereum-fetcher/internal/server"
	"ethereum-fetcher/internal/store"
	"ethereum-fetcher/internal/store/pg"

	"github.com/gorilla/mux"
	"github.com/spf13/viper"
	"go.uber.org/dig"
)

// SetupContainer registers all dependencies into the Dig container.
func SetupContainer(container *dig.Container) error {
	err := container.Provide(NewViper)
	if err != nil {
		return err
	}

	err = container.Provide(NewRouter)
	if err != nil {
		return err
	}

	err = container.Provide(NewContextWithCancel)
	if err != nil {
		return err
	}

	err = container.Provide(NewStore)
	if err != nil {
		return err
	}

	err = container.Provide(NewEthNode)
	if err != nil {
		return err
	}

	err = container.Provide(NewAppService)
	if err != nil {
		return err
	}

	err = container.Provide(NewEndpoint)
	if err != nil {
		return err
	}

	err = container.Provide(NewWebServer)
	if err != nil {
		return err
	}

	return nil
}

func NewViper() *viper.Viper {
	return cmd.NewViper()
}

func NewRouter() *mux.Router {
	return mux.NewRouter()
}

func NewContextWithCancel() (ctx context.Context, cancel context.CancelFunc) {
	return context.WithCancel(context.Background())
}

func NewStore(ctx context.Context, vp *viper.Viper) (store.StorageProvider, error) {
	return pg.NewStore(ctx, vp)
}

func NewEthNode(ctx context.Context, vp *viper.Viper) network.EthereumProvider {
	return network.NewEthNode(ctx, vp)
}

func NewAppService(ctx context.Context, vp *viper.Viper, st store.StorageProvider, net network.EthereumProvider,
) app.ServiceProvider {
	return app.NewService(ctx, vp, st, net)
}

func NewEndpoint(ctx context.Context, vp *viper.Viper, ap app.ServiceProvider) server.EndPointProvider {
	return server.NewEndPoint(ctx, vp, ap)
}

func NewWebServer(ctx context.Context, router *mux.Router,
	endPointProvider server.EndPointProvider) *server.WebServer {
	return server.NewServer(ctx, router, endPointProvider)
}
