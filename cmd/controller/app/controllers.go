package app

import (
	"context"
	"github.com/edgewize/edgeQ/pkg/controllers/imdeployment"
	"github.com/edgewize/edgeQ/pkg/controllers/imservice"
	"github.com/edgewize/edgeQ/pkg/controllers/imtemplate"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

type RegisterController interface {
	SetupWithManager(mgr ctrl.Manager) error
}

func addControllers(ctx context.Context, mgr manager.Manager) (err error) {
	controllers := []RegisterController{
		&imdeployment.Reconciler{},
		&imservice.Reconciler{},
		&imtemplate.Reconciler{},
	}

	for _, controller := range controllers {
		if err = controller.SetupWithManager(mgr); err != nil {
			return
		}
	}

	return
}
