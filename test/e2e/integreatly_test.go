package e2e

import (
	"bytes"
	goctx "context"
	"encoding/json"
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/remotecommand"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"testing"
	"time"

	routev1 "github.com/openshift/api/route/v1"

	"github.com/integr8ly/integreatly-operator/pkg/apis"

	operator "github.com/integr8ly/integreatly-operator/pkg/apis/integreatly/v1alpha1"

	framework "github.com/operator-framework/operator-sdk/pkg/test"
	"github.com/operator-framework/operator-sdk/pkg/test/e2eutil"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var (
	retryInterval                    = time.Second * 5
	timeout                          = time.Second * 60
	deploymentRetryInterval          = time.Second * 30
	deploymentTimeout                = time.Minute * 20
	cleanupRetryInterval             = time.Second * 1
	cleanupTimeout                   = time.Second * 5
	installationCleanupRetryInterval = time.Second * 20
	installationCleanupTimeout       = time.Minute * 4 //Longer timeout required to allow for finalizers to execute
	intlyNamespacePrefix             = "intly-"
	installationName                 = "e2e-managed-installation"
	bootstrapStage                   = "bootstrap"
	monitoringStage                  = "monitoring"
	authenticationStage              = "authentication"
	productsStage                    = "products"
	solutionExplorerStage            = "solution-explorer"
)

func TestIntegreatly(t *testing.T) {

	logf.SetLogger(logf.ZapLogger(true))

	installationList := &operator.InstallationList{}
	err := framework.AddToFrameworkScheme(apis.AddToScheme, installationList)
	if err != nil {
		t.Fatalf("failed to add custom resource scheme to framework: %v", err)
	}
	// run subtests
	t.Run("integreatly", func(t *testing.T) {
		t.Run("Cluster", IntegreatlyCluster)
	})

}

func waitForProductDeployment(t *testing.T, f *framework.Framework, ctx *framework.TestCtx, product, deploymentName string) error {
	namespace := intlyNamespacePrefix + product
	t.Logf("Checking %s:%s", namespace, deploymentName)

	start := time.Now()
	err := e2eutil.WaitForDeployment(t, f.KubeClient, namespace, deploymentName, 1, deploymentRetryInterval, deploymentTimeout)
	if err != nil {
		return err
	}

	end := time.Now()
	elapsed := end.Sub(start)

	t.Logf("%s:%s up, waited %d", namespace, deploymentName, elapsed)
	return nil
}

func integreatlyMonitoringTest(t *testing.T, f *framework.Framework, ctx *framework.TestCtx) error {

	// Define the json output of the prometheus api call
	type Labels struct {
		Alertname string `json:"alertname,omitempty"`
		Severity string  `json:"severity,omitempty"`
	}

	type Annotations struct {
		Message string `json:"message,omitempty"`
	}

	type Alerts struct {
		Labels Labels `json:"labels,omitempty"`
		State string `json:"state,omitempty"`
		Annotations Annotations `json:"annotations,omitempty"`
		ActiveAt string `json:"activeAt,omitempty"`
		Value string `json:"value,omitempty"`
	}

	type Data struct{
		Alerts []Alerts `json:"alerts,omitempty"`
	}

	type Output struct{
		Status string `json:"status"`
		Data Data `json:"data"`

	}

	output, err := execToPod("curl localhost:9090/api/v1/alerts",
							"prometheus-application-monitoring-0",
							"integreatly-middleware-monitoring",
							"prometheus", f )
	if err != nil {
		return fmt.Errorf("failed to exec to pod: %s" , err)
	}

	// just log it for now, once the resources are in the operator we can actually use it
	t.Logf("output: %s" , output)

	// dummy failure and success strings for testing

	// dms = true firing and pending alerts
	failure := `{"status":"success","data":{"alerts":[{"labels":{"alertname":"ESNotReady","severity":"warning"},"annotations":{"message":"Not all Elastic Search replication controllers are in a ready state."},"state":"firing","activeAt":"2019-11-20T10:02:27.667374029Z","value":"3e+00"},{"labels":{"alertname":"ClusterSchedulableMemoryLow","severity":"warning"},"annotations":{"message":"The cluster has 99% of memory requested and unavailable for scheduling for longer than 15 minutes."},"state":"pending","activeAt":"2019-11-20T10:02:27.667374029Z","value":"9.881767105240749e+01"},{"labels":{"alertname":"DeadMansSwitch","severity":"none"},"annotations":{"description":"This is a DeadMansSwitch meant to ensure that the entire Alerting pipeline is functional.","summary":"Alerting DeadMansSwitch"},"state":"firing","activeAt":"2019-11-11T16:09:50.701028455Z","value":"1e+00"}]}}`

	// dms = true no firing or pending alerts
	//success := `{"status":"success","data":{"alerts":[{"labels":{"alertname":"DeadMansSwitch","severity":"none"},"annotations":{"description":"This is a DeadMansSwitch meant to ensure that the entire Alerting pipeline is functional.","summary":"Alerting DeadMansSwitch"},"state":"firing","activeAt":"2019-11-11T16:09:50.701028455Z","value":"1e+00"}]}}`


	var promApiCallOutput Output
	err = json.Unmarshal([]byte(failure), &promApiCallOutput)
	if err != nil{
		t.Logf("Failed to unmarshall json: %s", err)
	}

	// Check if any alerts other than DeadMansSwitch are firing or pending
	var firingalerts []string
	var pendingalerts []string
	var deadmanswitchfiring = false
	for a := 0; a < len(promApiCallOutput.Data.Alerts) ; a++{
		if promApiCallOutput.Data.Alerts[a].Labels.Alertname == "DeadMansSwitch"{
			deadmanswitchfiring = true
		}
		if promApiCallOutput.Data.Alerts[a].Labels.Alertname != "DeadMansSwitch" {
			if promApiCallOutput.Data.Alerts[a].State == "firing" {
				firingalerts = append(firingalerts, promApiCallOutput.Data.Alerts[a].Labels.Alertname)
			}
			if promApiCallOutput.Data.Alerts[a].State == "pending"{
				pendingalerts = append(pendingalerts, promApiCallOutput.Data.Alerts[a].Labels.Alertname)
			}
		}
	}

	var status []string
	if len(firingalerts) > 0 {
		falert := fmt.Sprint(string(len(firingalerts))+ "Firing alerts: ", firingalerts)
		status = append(status, falert)
	}
	if len(pendingalerts) > 0 {
		palert := fmt.Sprint(string(len(pendingalerts))+ "Pending alerts: ", pendingalerts)
		status = append(status, palert)
	}
	if deadmanswitchfiring == false{
		dms := fmt.Sprint("DeadMansSwitch is not firing: ", deadmanswitchfiring)
		status = append(status, dms)
	}

	if len(status) > 0 {
		return fmt.Errorf("alert tests failed: %s", status)
	}

	t.Logf("No unexpected alerts found")
	return nil
}

func execToPod(command string, podname string, namespace string, container string, f *framework.Framework ) (string, error){
	req := f.KubeClient.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podname).
		Namespace(namespace).
		SubResource("exec").
		Param("container", container)
	scheme := runtime.NewScheme()
	if err := v1.AddToScheme(scheme); err != nil {
		return "", fmt.Errorf("error adding to scheme: %v", err)
	}
	parameterCodec := runtime.NewParameterCodec(scheme)
	req.VersionedParams(&v1.PodExecOptions{
		Container: 	container,
		Command:	strings.Fields(command),
		Stdin:		false,
		Stdout:     true,
		Stderr:		true,
		TTY:		false,
	}, parameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(f.KubeConfig, "POST", req.URL())
	if err != nil {
		return "", fmt.Errorf("error while creating Executor: %v", err)
	}

	var stdout, stderr  bytes.Buffer
	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  nil,
		Stdout: &stdout,
		Stderr: &stderr,
		Tty:    false,
	})
	if err != nil {
		return "", fmt.Errorf("error in Stream: %v", err)
	}

	return stdout.String(), nil
}


func getConfigMap (name string, namespace string, f *framework.Framework) (map[string]string, error) {
	configmap := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	key := client.ObjectKey{
		Name:      configmap.GetName(),
		Namespace: configmap.GetNamespace(),
	}
	err := f.Client.Get(goctx.TODO(), key, configmap)
	if err != nil {
		return map[string]string{}, fmt.Errorf("could not get configmap: %configmapname", err)
	}

	return configmap.Data, nil
}
func integreatlyManagedTest(t *testing.T, f *framework.Framework, ctx *framework.TestCtx) error {
	namespace, err := ctx.GetNamespace()
	if err != nil {
		return fmt.Errorf("could not get namespace: %deploymentName", err)
	}

	consoleRouteCR := &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "console",
			Namespace: "openshift-console",
		},
	}
	key := client.ObjectKey{
		Name:      consoleRouteCR.GetName(),
		Namespace: consoleRouteCR.GetNamespace(),
	}
	err = f.Client.Get(goctx.TODO(), key, consoleRouteCR)
	if err != nil {
		return fmt.Errorf("could not get console route: %deploymentName", err)
	}
	masterUrl := consoleRouteCR.Status.Ingress[0].Host
	routingSubdomain := consoleRouteCR.Status.Ingress[0].RouterCanonicalHostname

	t.Logf("Creating installation CR with routingSubdomain:%s, masterUrl:%s\n", routingSubdomain, masterUrl)

	// create installation custom resource
	managedInstallation := &operator.Installation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      installationName,
			Namespace: namespace,
		},
		Spec: operator.InstallationSpec{
			Type:             "managed",
			NamespacePrefix:  intlyNamespacePrefix,
			RoutingSubdomain: routingSubdomain,
			MasterURL:        masterUrl,
			SelfSignedCerts:  true,
		},
	}
	// use TestCtx's create helper to create the object and add a cleanup function for the new object
	err = f.Client.Create(goctx.TODO(), managedInstallation, &framework.CleanupOptions{TestContext: ctx, Timeout: installationCleanupTimeout, RetryInterval: installationCleanupRetryInterval})
	if err != nil {
		return err
	}

	// wait for bootstrap phase to complete (5 minutes timeout)
	err = waitForInstallationStageCompletion(t, f, namespace, deploymentRetryInterval, deploymentTimeout, bootstrapStage)
	if err != nil {
		return err
	}

	// wait for middleware-monitoring to deploy
	err = waitForProductDeployment(t, f, ctx, "middleware-monitoring", "application-monitoring-operator")
	if err != nil {
		return err
	}

	// wait for authentication phase to complete (15 minutes timeout)
	err = waitForInstallationStageCompletion(t, f, namespace, deploymentRetryInterval, deploymentTimeout, monitoringStage)
	if err != nil {
		return err
	}

	// wait for keycloak-operator to deploy
	err = waitForProductDeployment(t, f, ctx, "rhsso", "keycloak-operator")
	if err != nil {
		return err
	}

	// wait for authentication phase to complete (15 minutes timeout)
	err = waitForInstallationStageCompletion(t, f, namespace, deploymentRetryInterval, deploymentTimeout, authenticationStage)
	if err != nil {
		return err
	}

	//Product Stage - verify operators deploy
	products := map[string]string{
		"3scale":                  "3scale-operator",
		"amq-online":              "enmasse-operator",
		"codeready-workspaces":    "codeready-operator",
		"fuse":                    "syndesis-operator",
		"launcher":                "launcher-operator",
		"mdc":                     "mobile-developer-console-operator",
		"mobile-security-service": "mobile-security-service-operator",
		"user-sso":                "keycloak-operator",
		"ups":                     "unifiedpush-operator",
	}
	for product, deploymentName := range products {
		err = waitForProductDeployment(t, f, ctx, product, deploymentName)
		if err != nil {
			return err
		}
	}

	// wait for products phase to complete (5 minutes timeout)
	err = waitForInstallationStageCompletion(t, f, namespace, deploymentRetryInterval, deploymentTimeout*2, productsStage)
	if err != nil {
		return err
	}

	// wait for solution-explorer operator to deploy
	err = waitForProductDeployment(t, f, ctx, "solution-explorer", "tutorial-web-app-operator")
	if err != nil {
		return err
	}

	// wait for solution-explorer phase to complete (10 minutes timeout)
	err = waitForInstallationStageCompletion(t, f, namespace, deploymentRetryInterval, deploymentTimeout, solutionExplorerStage)
	if err != nil {
		return err
	}
	return err
}

func waitForInstallationStageCompletion(t *testing.T, f *framework.Framework, namespace string, retryInterval, timeout time.Duration, phase string) error {
	err := wait.Poll(retryInterval, timeout, func() (done bool, err error) {
		installation := &operator.Installation{}
		err = f.Client.Get(goctx.TODO(), types.NamespacedName{Name: installationName, Namespace: namespace}, installation)
		if err != nil {
			if apierrors.IsNotFound(err) {
				t.Logf("Waiting for availability of %s installation\n", installationName)
				return false, nil
			}
			return false, err
		}

		phaseStatus := fmt.Sprintf("%#v", installation.Status.Stages[operator.StageName(phase)])
		if strings.Contains(phaseStatus, "completed") {
			return true, nil
		}

		t.Logf("Waiting for completion of %s\n", phase)
		return false, nil
	})
	if err != nil {
		return err
	}
	t.Logf("%s phase completed \n", phase)
	return nil
}

func IntegreatlyCluster(t *testing.T) {
	ctx := framework.NewTestCtx(t)
	defer ctx.Cleanup()
	//err := ctx.InitializeClusterResources(&framework.CleanupOptions{TestContext: ctx, Timeout: cleanupTimeout, RetryInterval: cleanupRetryInterval})
	//if err != nil {
	//	t.Fatalf("failed to initialize cluster resources: %v", err)
	//}
	//t.Log("Initialized cluster resources")
	//namespace, err := ctx.GetNamespace()
	//if err != nil {
	//	t.Fatal(err)
	//}
	// get global framework variables
	f := framework.Global
	// wait for integreatly-operator to be ready
	//err = e2eutil.WaitForOperatorDeployment(t, f.KubeClient, namespace, "integreatly-operator", 1, retryInterval, timeout)
	//if err != nil {
	//	t.Fatal(err)
	//}
	// check that all of the operators deploy and all of the installation phases complete
	//if err = integreatlyManagedTest(t, f, ctx); err != nil {
	//	t.Fatal(err)
	//}
	if err := integreatlyMonitoringTest(t, f, ctx); err != nil {
		t.Fatal(err)
	}
}
