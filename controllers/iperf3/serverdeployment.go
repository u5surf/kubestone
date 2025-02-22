/*
Copyright 2019 The xridge kubestone contributors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package iperf3

import (
	"strconv"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	perfv1alpha1 "github.com/xridge/kubestone/api/v1alpha1"

	"github.com/firepear/qsplit"
)

// Iperf3ServerPort is the TCP or UDP port where
// the iperf3 server deployment and service listens
const Iperf3ServerPort = 5201

// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;create;delete;watch

func serverDeploymentName(cr *perfv1alpha1.Iperf3) string {
	return cr.Name
}

// NewServerDeployment create a iperf3 server deployment from the
// provided Iperf3 Benchmark Definition.
func NewServerDeployment(cr *perfv1alpha1.Iperf3) *appsv1.Deployment {
	replicas := int32(1)

	labels := map[string]string{
		"app":               "iperf3",
		"kubestone-cr-name": cr.Name,
	}
	// Let's be nice and don't mutate CRs label field
	for k, v := range cr.Spec.ServerConfiguration.PodLabels {
		labels[k] = v
	}

	iperfCmdLineArgs := []string{
		"--server",
		"--port", strconv.Itoa(Iperf3ServerPort)}

	protocol := corev1.Protocol(corev1.ProtocolTCP)
	if cr.Spec.UDP {
		iperfCmdLineArgs = append(iperfCmdLineArgs, "--udp")
		protocol = corev1.Protocol(corev1.ProtocolUDP)
	}

	iperfCmdLineArgs = append(iperfCmdLineArgs,
		qsplit.ToStrings([]byte(cr.Spec.ClientConfiguration.CmdLineArgs))...)

	deployment := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serverDeploymentName(cr),
			Namespace: cr.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Replicas: &replicas,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					ImagePullSecrets: []corev1.LocalObjectReference{
						{
							Name: cr.Spec.Image.PullSecret,
						},
					},
					Containers: []corev1.Container{
						{
							Name:            "server",
							Image:           cr.Spec.Image.Name,
							ImagePullPolicy: corev1.PullPolicy(cr.Spec.Image.PullPolicy),
							Command:         []string{"iperf3"},
							Args:            iperfCmdLineArgs,
							Ports: []corev1.ContainerPort{
								{
									Name:          "iperf-server",
									ContainerPort: Iperf3ServerPort,
									Protocol:      protocol,
								},
							},
							/* -- Causing iperf3 server to exit with 'too many errors'
							ReadinessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									TCPSocket: &corev1.TCPSocketAction{
										Port: intstr.FromInt(Iperf3ServerPort),
									},
								},
								InitialDelaySeconds: 5,
								TimeoutSeconds:      5,
								PeriodSeconds:       5,
							},
							*/
						},
					},
					Affinity:     &cr.Spec.ServerConfiguration.PodScheduling.Affinity,
					Tolerations:  cr.Spec.ServerConfiguration.PodScheduling.Tolerations,
					NodeSelector: cr.Spec.ServerConfiguration.PodScheduling.NodeSelector,
					NodeName:     cr.Spec.ServerConfiguration.PodScheduling.NodeName,
					HostNetwork:  cr.Spec.ServerConfiguration.HostNetwork,
				},
			},
		},
	}

	return &deployment
}

func (r *Reconciler) serverDeploymentReady(cr *perfv1alpha1.Iperf3) (ready bool, err error) {
	// TODO: Move this to k8s.client
	ready, err = false, nil
	deployment, err := r.K8S.Clientset.AppsV1().Deployments(cr.Namespace).Get(cr.Name, metav1.GetOptions{})
	if err != nil {
		return ready, err
	}

	ready = deployment.Status.ReadyReplicas == *deployment.Spec.Replicas

	return ready, err
}
