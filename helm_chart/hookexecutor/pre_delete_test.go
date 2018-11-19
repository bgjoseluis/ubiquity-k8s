package hookexecutor

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	_ "github.com/IBM/ubiquity-k8s/cmd/hook-executor/logger"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	fakekubeclientset "k8s.io/client-go/kubernetes/fake"
	testcore "k8s.io/client-go/testing"
)

var test_pvYaml = `
apiVersion: v1
kind: PersistentVolume
metadata:
  name: ibm-ubiquity-db
spec:
  accessModes:
  - ReadWriteOnce
  capacity:
    storage: 20Gi
  claimRef:
    apiVersion: v1
    kind: PersistentVolumeClaim
    name: ibm-ubiquity-db
    namespace: ubiquity
  persistentVolumeReclaimPolicy: Delete
  storageClassName: gold
status:
  phase: Bound
`

var test_pvcYaml = `
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  labels:
    pv-name: ibm-ubiquity-db
  name: ibm-ubiquity-db
  namespace: ubiquity
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 20Gi
  volumeName: ibm-ubiquity-db
status:
  accessModes:
  - ReadWriteOnce
  capacity:
    storage: 20Gi
  phase: Bound
`

var test_deployYaml = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ubiquity-db
  namespace: ubiquity
spec:
  replicas: 1
  selector:
    matchLabels:
      app: ubiquity-db
      product: ibm-storage-enabler-for-containers
  template:
    metadata:
      labels:
        app: ubiquity-db
        product: ibm-storage-enabler-for-containers
    spec:
      containers:
      - image: ibmcom/ibm-storage-enabler-for-containers-db:1.2.0
        name: ubiquity-db
        volumeMounts:
        - mountPath: /var/lib/postgresql/data
          name: ibm-ubiquity-db
          subPath: ibm-ubiquity
      volumes:
      - name: ibm-ubiquity-db
        persistentVolumeClaim:
          claimName: ibm-ubiquity-db
`

var test_podYaml = `
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: ubiquity-db
    product: ibm-storage-enabler-for-containers
  name: ubiquity-db-868c4cb89d-b2xvs
  namespace: ubiquity
spec:
  containers:
  - image: ibmcom/ibm-storage-enabler-for-containers-db:1.2.0
    name: ubiquity-db
    volumeMounts:
    - mountPath: /var/lib/postgresql/data
      name: ibm-ubiquity-db
      subPath: ibm-ubiquity
    - mountPath: /var/run/secrets/kubernetes.io/serviceaccount
      name: default-token-lrwfk
      readOnly: true
  volumes:
  - name: ibm-ubiquity-db
    persistentVolumeClaim:
      claimName: ibm-ubiquity-db
  - name: default-token-lrwfk
    secret:
      defaultMode: 420
      secretName: default-token-lrwfk
`

var test_one = int32(1)
var test_zero = int32(0)

var _ = Describe("PreDelete", func() {

	var e Executor
	var kubeClient *fakekubeclientset.Clientset
	var pvc *v1.PersistentVolumeClaim
	var pv *v1.PersistentVolume
	var deploy *appsv1.Deployment
	var pod *v1.Pod
	var stopped chan bool

	BeforeEach(func() {

		stopped = make(chan bool)

		pvcObj, _ := FromYaml([]byte(test_pvcYaml))
		pvc = pvcObj.(*v1.PersistentVolumeClaim)

		pvObj, _ := FromYaml([]byte(test_pvYaml))
		pv = pvObj.(*v1.PersistentVolume)

		deployObj, _ := FromYaml([]byte(test_deployYaml))
		deploy = deployObj.(*appsv1.Deployment)

		podObj, _ := FromYaml([]byte(test_podYaml))
		pod = podObj.(*v1.Pod)

		kubeClient = fakekubeclientset.NewSimpleClientset(pvc, pv, deploy, pod)

		e = PreDeleteExecutor(kubeClient)
	})

	AfterEach(func() {
		close(stopped)
	})

	Describe("test Execute", func() {

		Context("delete UbiquityDB Pods", func() {

			BeforeEach(func() {
				deploy, err := kubeClient.AppsV1().Deployments(deploy.Namespace).Get(deploy.Name, metav1.GetOptions{})
				Ω(err).ShouldNot(HaveOccurred())
				Expect(deploy.Spec.Replicas).To(Equal(&test_one))

				go func() {
					podWatcher := watch.NewFake()
					kubeClient.PrependWatchReactor("pods", testcore.DefaultWatchReactor(podWatcher, nil))

					time.Sleep(40 * time.Millisecond)
					podWatcher.Delete(pod)
				}()

				go func() {
					err := e.(*preDeleteExecutor).deleteUbiquityDBPods()
					Ω(err).ShouldNot(HaveOccurred())
					stopped <- true
				}()
			})

			It("should be deleted successfully", func(done Done) {
				Expect(<-stopped).To(BeTrue())
				deploy, err := kubeClient.AppsV1().Deployments(deploy.Namespace).Get(deploy.Name, metav1.GetOptions{})
				Ω(err).ShouldNot(HaveOccurred())

				// we only check if the replicas is updated since the fake server won't trigger any pod changes.
				Expect(deploy.Spec.Replicas).To(Equal(&test_zero))

				close(done)
			})
		})

		Context("delete UbiquityDB pvc", func() {

			BeforeEach(func() {

				// pvc and pv exist on API Server before action
				_, err := kubeClient.CoreV1().PersistentVolumeClaims(pvc.Namespace).Get(pvc.Name, metav1.GetOptions{})
				Ω(err).ShouldNot(HaveOccurred())

				_, err = kubeClient.CoreV1().PersistentVolumes().Get(pv.Name, metav1.GetOptions{})
				Ω(err).ShouldNot(HaveOccurred())

				go func() {
					pvcWatcher := watch.NewFake()
					kubeClient.PrependWatchReactor("persistentvolumeclaims", testcore.DefaultWatchReactor(pvcWatcher, nil))

					time.Sleep(40 * time.Millisecond)
					pvcWatcher.Delete(pvc)
				}()

				go func() {
					pvWatcher := watch.NewFake()
					kubeClient.PrependWatchReactor("persistentvolumes", testcore.DefaultWatchReactor(pvWatcher, nil))

					time.Sleep(50 * time.Millisecond)
					pvWatcher.Delete(pv)
				}()

				go func() {
					e.(*preDeleteExecutor).deleteUbiquityDBPvc()
					// The fake server won't delete pv in cascade.
					kubeClient.CoreV1().PersistentVolumes().Delete(pv.Name, nil)
					stopped <- true
				}()
			})

			It("should delete pvc and pv", func(done Done) {
				Expect(<-stopped).To(BeTrue())
				_, err := kubeClient.CoreV1().PersistentVolumeClaims(pvc.Namespace).Get(pvc.Name, metav1.GetOptions{})
				Expect(apierrors.IsNotFound(err)).To(BeTrue())

				_, err = kubeClient.CoreV1().PersistentVolumes().Get(pv.Name, metav1.GetOptions{})
				Expect(apierrors.IsNotFound(err)).To(BeTrue())

				close(done)
			})
		})
	})
})