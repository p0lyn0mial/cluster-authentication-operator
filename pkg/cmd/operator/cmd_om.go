package operator

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/utils/clock"

	"github.com/openshift/cluster-authentication-operator/pkg/operator"
)

func RunOperatorInOpenshiftManagerMode(ctx context.Context, input *operator.OpenshiftManagerInput) error {
	authenticationOperatorInput, err := operator.CreateOperatorInputFromOM(ctx, input)
	if err != nil {
		return fmt.Errorf("unable to configure operator input: %w", err)
	}
	operatorStarter, err := operator.CreateOperatorStarter(ctx, authenticationOperatorInput)
	if err != nil {
		return fmt.Errorf("unable to configure operators: %w", err)
	}

	controllersToRun := []string{
		"om-demo",
	}
	if err := operatorStarter.StartNamedControllers(ctx, controllersToRun); err != nil {
		return fmt.Errorf("unable to start operators: %w", err)
	}

	<-ctx.Done()
	return nil
}

type runOperatorInOpenshiftManagerModeFunc func(ctx context.Context, input *operator.OpenshiftManagerInput) error

func NewOpenshiftManagerCommand() *cobra.Command {
	f := newOpenshiftManagerFlags(RunOperatorInOpenshiftManagerMode)

	cmd := &cobra.Command{
		Use:   "om-operator",
		Short: "Run the auth operator in Openshift Manager compatible mode",

		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			if err := f.Validate(); err != nil {
				return err
			}
			o, err := f.ToOptions(ctx)
			if err != nil {
				return err
			}
			if err := o.Run(ctx); err != nil {
				return err
			}
			return nil
		},
	}

	f.BindFlags(cmd.Flags())

	return cmd
}

type openshiftManagerOptions struct {
	operatorStartFunc runOperatorInOpenshiftManagerModeFunc
	input             *operator.OpenshiftManagerInput
}

func (o *openshiftManagerOptions) Run(ctx context.Context) error {
	return o.operatorStartFunc(ctx, o.input)
}

type openshiftManagerFlags struct {
	operatorStartFunc runOperatorInOpenshiftManagerModeFunc

	kubeConfigPath             string
	guestClusterKubeConfigPath string
}

func (f *openshiftManagerFlags) BindFlags(flags *pflag.FlagSet) {
	flags.StringVar(&f.kubeConfigPath, "kubeconfig", f.kubeConfigPath, "The directory where the kubeconfig for the management cluster is stored.")
	flags.StringVar(&f.guestClusterKubeConfigPath, "guest-kubeconfig", f.guestClusterKubeConfigPath, "The directory where the kubeconfig for the guest cluster is stored.")
}

func (f *openshiftManagerFlags) Validate() error {
	if len(f.kubeConfigPath) == 0 {
		return fmt.Errorf("--kubeconfig is required")
	}
	if len(f.guestClusterKubeConfigPath) == 0 {
		return fmt.Errorf("--guest-kubeconfig is required")
	}
	return nil
}

func (f *openshiftManagerFlags) ToOptions(ctx context.Context) (*openshiftManagerOptions, error) {
	managementClusterKubeConfig, err := clientcmd.BuildConfigFromFlags("", f.kubeConfigPath)
	if err != nil {
		return nil, err
	}
	guestClusterKubeConfig, err := clientcmd.BuildConfigFromFlags("", f.guestClusterKubeConfigPath)
	if err != nil {
		return nil, err
	}

	// force json
	managementClusterKubeConfig.AcceptContentTypes = "application/json"
	managementClusterKubeConfig.ContentType = "application/json"

	guestClusterKubeConfig.AcceptContentTypes = "application/json"
	guestClusterKubeConfig.ContentType = "application/json"

	return &openshiftManagerOptions{
		operatorStartFunc: f.operatorStartFunc,
		input: &operator.OpenshiftManagerInput{
			ManagementClusterKubeConfig: managementClusterKubeConfig,
			GuestClusterKubeConfig:      guestClusterKubeConfig,
			Clock:                       clock.RealClock{},
		},
	}, nil
}

func newOpenshiftManagerFlags(operatorStartFunc runOperatorInOpenshiftManagerModeFunc) *openshiftManagerFlags {
	return &openshiftManagerFlags{operatorStartFunc: operatorStartFunc}
}
