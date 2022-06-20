package cmd

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/caihcloud/node-annotator/conf"
	"github.com/caihcloud/node-annotator/pkg/annotator"
	"github.com/caihcloud/node-annotator/pkg/client"
	"github.com/caihcloud/node-annotator/pkg/prometheus"
	"github.com/google/uuid"

	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/klog"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func init() {
	rootCmd.PersistentFlags().StringVar(&conf.KubeconfigPath, "config", "", "kubeconfig file path set for debug")
	rootCmd.PersistentFlags().StringVar(&conf.PrometheusUrl, "prometheus-url", "", "prometheus url")
	rootCmd.PersistentFlags().StringVar(&conf.PushGatewayUrl, "pushgateway-url", "", "pushgateway url")
	rootCmd.PersistentFlags().StringVar(&conf.DynamicSchedulerName, "scheduler-name", "caihcloud-scheduler", "scheduler name")
	rootCmd.PersistentFlags().StringVar(&conf.LeaseLockName, "lease-lock-name", "node-annotator", "lease-lock-name")
	rootCmd.PersistentFlags().StringVar(&conf.LeaseLockNamespace, "lease-lock-namespace", "monitor", "lease-lock-namespace")
	rootCmd.PersistentFlags().StringVar(&conf.ConfigFile, "annotator-config", "/config/annotator-config.yaml", "annotator-config")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		klog.Errorln(os.Stderr, err)
		os.Exit(1)
	}
}

var LeaseLockID string

var rootCmd = &cobra.Command{
	Use:   "node-annotator",
	Short: "node annotator for caihcloud scheduler",
	Args:  cobra.MinimumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		klog.Infof("Use kubeconf: [%s]  prometheus-url: [%s]  pushgateway-url: [%s]", conf.KubeconfigPath, conf.PrometheusUrl, conf.PushGatewayUrl)
		if conf.PrometheusUrl == "" {
			klog.Fatal("must set Prometheus Url using --prometheus-url {prometheus url}")
		}

		LeaseLockID = uuid.New().String()
		client.InitClientSet()

		run := func(ctx context.Context) {
			annotator.PreRun()
			if conf.PushGatewayUrl != "" {
				prometheus.InitPushGatewayCounter()
				go prometheus.InformerPodScheduler()
			}
			annotator.Run()
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-ch
			klog.Infoln("Received termination, signaling shutdown")
			cancel()
		}()

		lock := &resourcelock.LeaseLock{
			LeaseMeta: metav1.ObjectMeta{
				Name:      conf.LeaseLockName,
				Namespace: conf.LeaseLockNamespace,
			},
			Client: client.K8sClientSet.CoordinationV1(),
			LockConfig: resourcelock.ResourceLockConfig{
				Identity: LeaseLockID,
			},
		}

		leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
			Lock:            lock,
			ReleaseOnCancel: true,
			LeaseDuration:   30 * time.Second, //租约时间
			RenewDeadline:   15 * time.Second, //更新租约的
			RetryPeriod:     5 * time.Second,  //非leader节点重试时间
			Callbacks: leaderelection.LeaderCallbacks{
				OnStartedLeading: func(ctx context.Context) {
					//变为leader执行的业务代码
					run(ctx)
				},
				OnStoppedLeading: func() {
					// 进程退出
					klog.Infof("leader lost: %s", LeaseLockID)
					os.Exit(0)
				},
				OnNewLeader: func(identity string) {
					//当产生新的leader后执行的方法
					if identity == LeaseLockID {
						klog.Infof("i am leader now: %s", identity)
						return
					}
					klog.Infof("new leader elected: %s, wait...", identity)
				},
			},
		})

	},
}
