package main

import (
	goflag "flag"
	flag "github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/util/wait"
	"log"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/kubectl/pkg/util/templates"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/klog/v2"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/ecordell/rukpak/pkg/signals"
	"github.com/ecordell/rukpak/pkg/apis/rukpak/informers/externalversions"
	"github.com/ecordell/rukpak/pkg/apis/rukpak/clientset/versioned"
)

var logFlushFreq = flag.Duration("log-flush-frequency", 5*time.Second, "Maximum number of seconds between log flushes")

func main() {
	ctx := signals.Context()
	cmd := &cobra.Command{
		Use:   "rukpak",
		Short: "rukpak pulls and applies bundles on clusters",
		Long: templates.LongDesc(`rukpak pulls and applies bundles on clusters`),

	}
	kubeConfigFlags := genericclioptions.NewConfigFlags(true).WithDeprecatedPasswordFlag()
	kubeConfigFlags.AddFlags(cmd.PersistentFlags())
	matchVersionKubeConfigFlags := cmdutil.NewMatchVersionFlags(kubeConfigFlags)
	matchVersionKubeConfigFlags.AddFlags(cmd.PersistentFlags())
	f := cmdutil.NewFactory(matchVersionKubeConfigFlags)
	var kflags goflag.FlagSet
	klog.InitFlags(&kflags)
	cmd.PersistentFlags().AddGoFlagSet(&kflags)
	go wait.Until(klog.Flush, *logFlushFreq, wait.NeverStop)
	defer klog.Flush()

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		restClient, err := f.RESTClient()
		if err != nil {
			klog.Fatalf("Error building rest client: %s", err.Error())
		}

		kubeClient, err := f.KubernetesClientSet()
		if err != nil {
			klog.Fatalf("Error building kubernetes clientset: %s", err.Error())
		}

		kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, time.Second*30)

		rukpakClient := versioned.New(restClient)
		rukpakInformerFactory := externalversions.NewSharedInformerFactory(rukpakClient, time.Second*30)

		controller := NewController(kubeClient, rukpakClient,
			kubeInformerFactory.Apps().V1().Deployments(),
			rukpakInformerFactory.Rukpak().V1alpha1().Bundles())

		kubeInformerFactory.Start(ctx.Done())
		rukpakInformerFactory.Start(ctx.Done())
		if err = controller.Run(2, ctx.Done()); err != nil {
			klog.Fatalf("Error running controller: %s", err.Error())
		}
		return nil
	}

	if err := cmd.ExecuteContext(ctx); err != nil {
		log.Fatal(err)
	}
}

