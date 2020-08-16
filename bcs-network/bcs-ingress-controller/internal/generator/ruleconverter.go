/*
 * Tencent is pleased to support the open source community by making Blueking Container Service available.,
 * Copyright (C) 2019 THL A29 Limited, a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 * http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing, software distributed under,
 * the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the specific language governing permissions and
 * limitations under the License.
 */

package generator

import (
	"context"
	"fmt"

	k8scorev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/Tencent/bk-bcs/bcs-common/common/blog"
	networkextensionv1 "github.com/Tencent/bk-bcs/bcs-k8s/kubernetes/apis/networkextension/v1"
)

// RuleConverter rule converter
type RuleConverter struct {
	cli              client.Client
	lbIDs            []string
	ingressName      string
	ingressNamespace string
	rule             *networkextensionv1.IngressRule
}

// NewRuleConverter create rule converter
func NewRuleConverter(
	cli client.Client,
	lbIDs []string,
	ingressName string,
	ingressNamespace string,
	rule *networkextensionv1.IngressRule) *RuleConverter {

	return &RuleConverter{
		cli:              cli,
		lbIDs:            lbIDs,
		ingressName:      ingressName,
		ingressNamespace: ingressNamespace,
		rule:             rule,
	}
}

// DoConvert do convert action
func (rc *RuleConverter) DoConvert() ([]*networkextensionv1.Listener, error) {
	var retListeners []*networkextensionv1.Listener
	switch rc.rule.Protocol {
	case networkextensionv1.ProtocolHTTP, networkextensionv1.ProtocolHTTPS:
		for _, lbID := range rc.lbIDs {
			listener, err := rc.generate7LayerListener(lbID)
			if err != nil {
				return nil, err
			}
			retListeners = append(retListeners, listener)
		}
	case networkextensionv1.ProtocolTCP, networkextensionv1.ProtocolUDP:
		for _, lbID := range rc.lbIDs {
			listener, err := rc.generate4LayerListener(lbID)
			if err != nil {
				return nil, err
			}
			retListeners = append(retListeners, listener)
		}
	default:
		blog.Errorf("invalid protocol %s", rc.rule.Protocol)
		return nil, fmt.Errorf("invalid protocol %s", rc.rule.Protocol)
	}
	return retListeners, nil
}

func (rc *RuleConverter) generate7LayerListener(lbID string) (*networkextensionv1.Listener, error) {
	li := &networkextensionv1.Listener{}
	li.SetName(GetListenerName(lbID, rc.rule.Port))
	li.SetNamespace(rc.ingressNamespace)
	li.SetLabels(map[string]string{
		rc.ingressName:      networkextensionv1.LabelValueForIngressName,
		rc.ingressNamespace: networkextensionv1.LabelValueForIngressNamespace,
		networkextensionv1.LabelKeyForLoadbalanceID: lbID,
	})
	li.Spec.Port = rc.rule.Port
	li.Spec.Protocol = rc.rule.Protocol
	li.Spec.LoadbalancerID = lbID
	if rc.rule.ListenerAttribute != nil {
		li.Spec.ListenerAttribute = rc.rule.ListenerAttribute
	}
	if rc.rule.Certificate != nil {
		li.Spec.Certificate = rc.rule.Certificate
	}

	listenerRules, err := rc.generateListenerRule(rc.rule.Routes)
	if err != nil {
		return nil, err
	}
	li.Spec.Rules = listenerRules
	return li, nil
}

func (rc *RuleConverter) generateListenerRule(l7Routes []*networkextensionv1.Layer7Route) (
	[]*networkextensionv1.ListenerRule, error) {

	var retListenerRules []*networkextensionv1.ListenerRule
	for _, l7Route := range l7Routes {
		liRule := &networkextensionv1.ListenerRule{}
		liRule.Domain = l7Route.Domain
		liRule.Path = l7Route.Path
		targetGroup, err := rc.generateTargetGroup(rc.rule.Protocol, l7Route.Services)
		if err != nil {
			return nil, err
		}
		liRule.TargetGroup = targetGroup
		retListenerRules = append(retListenerRules, liRule)
	}
	return retListenerRules, nil
}

func (rc *RuleConverter) generate4LayerListener(lbID string) (*networkextensionv1.Listener, error) {
	li := &networkextensionv1.Listener{}
	li.SetName(GetListenerName(lbID, rc.rule.Port))
	li.SetNamespace(rc.ingressNamespace)
	li.SetLabels(map[string]string{
		rc.ingressName:      networkextensionv1.LabelValueForIngressName,
		rc.ingressNamespace: networkextensionv1.LabelValueForIngressNamespace,
		networkextensionv1.LabelKeyForLoadbalanceID: lbID,
	})
	li.Spec.Port = rc.rule.Port
	li.Spec.Protocol = rc.rule.Protocol
	li.Spec.LoadbalancerID = lbID
	if rc.rule.ListenerAttribute != nil {
		li.Spec.ListenerAttribute = rc.rule.ListenerAttribute
	}
	if rc.rule.Certificate != nil {
		li.Spec.Certificate = rc.rule.Certificate
	}

	targetGroup, err := rc.generateTargetGroup(rc.rule.Protocol, rc.rule.Services)
	if err != nil {
		return nil, err
	}
	li.Spec.TargetGroup = targetGroup
	return li, nil
}

func (rc *RuleConverter) generateTargetGroup(protocol string, routes []*networkextensionv1.ServiceRoute) (
	*networkextensionv1.ListenerTargetGroup, error) {

	var retBackends []*networkextensionv1.ListenerBackend
	for _, route := range routes {
		backends, err := rc.generateServiceBackendList(route)
		if err != nil {
			return nil, err
		}
		retBackends = mergeBackendList(retBackends, backends)
	}
	return &networkextensionv1.ListenerTargetGroup{
		TargetGroupProtocol: protocol,
		Backends:            retBackends,
	}, nil
}

func (rc *RuleConverter) generateServiceBackendList(svcRoute *networkextensionv1.ServiceRoute) (
	[]*networkextensionv1.ListenerBackend, error) {

	svc := &k8scorev1.Service{}
	err := rc.cli.Get(context.TODO(), k8stypes.NamespacedName{
		Namespace: svcRoute.ServiceNamespace,
		Name:      svcRoute.ServiceName,
	}, svc)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("get Service %s/%s failed, err %s",
			svcRoute.ServiceNamespace, svcRoute.ServiceName, err.Error())
	}
	var svcPort *k8scorev1.ServicePort
	for _, port := range svc.Spec.Ports {
		if port.Port == int32(svcRoute.ServicePort) {
			svcPort = &port
			break
		}
	}
	if svcPort == nil {
		blog.Warnf("port %d is not found in service %s/%s",
			svcRoute.ServicePort, svcRoute.ServiceNamespace, svcRoute.ServiceName)
		return nil, nil
	}

	if svcRoute.IsDirectConnect {
		backends, err := rc.getServiceBackendsWithoutSubsets(svcPort, svcRoute.ServiceNamespace,
			svcRoute.ServiceName, svcRoute.GetWeight())
		if err != nil {
			return nil, err
		}
		// to pod directly and no subset
		if len(svcRoute.Subsets) == 0 {
			return backends, nil
		}
		var retBackends []*networkextensionv1.ListenerBackend
		// to pod directly and have subset
		for _, subset := range svcRoute.Subsets {
			subsetBackends, err := rc.getSubsetBackends(svc, subset, backends)
			if err != nil {
				return nil, err
			}
			retBackends = mergeBackendList(retBackends, subsetBackends)
		}
		return retBackends, nil
	}
	// to node port
	retBackends, err := rc.getNodePortBackends(svc, svcPort)
	if err != nil {
		return nil, err
	}
	return retBackends, nil
}

func mergeBackendList(
	existedList, newList []*networkextensionv1.ListenerBackend) []*networkextensionv1.ListenerBackend {

	tmpMap := make(map[string]*networkextensionv1.ListenerBackend)
	for _, backend := range existedList {
		tmpMap[backend.IP] = backend
	}
	for _, backend := range newList {
		if _, ok := tmpMap[backend.IP]; !ok {
			existedList = append(existedList, backend)
		}
	}
	return existedList
}

func (rc *RuleConverter) getServiceBackendsWithoutSubsets(
	svcPort *k8scorev1.ServicePort, svcName, svcNamespace string, weight int) (
	[]*networkextensionv1.ListenerBackend, error) {

	eps := &k8scorev1.Endpoints{}
	err := rc.cli.Get(context.TODO(), k8stypes.NamespacedName{
		Namespace: svcNamespace,
		Name:      svcName,
	}, eps)
	if err != nil {
		return nil, fmt.Errorf("get endpoints %s failed, err %s", err.Error())
	}
	found := false
	var targetPort int
	var epsAddresses []k8scorev1.EndpointAddress
	for _, subset := range eps.Subsets {
		for _, port := range subset.Ports {
			if len(svcPort.Name) == 0 && port.Port == int32(svcPort.TargetPort.IntValue()) {
				targetPort = int(port.Port)
				found = true
				break
			}
			if len(svcPort.Name) != 0 && port.Name == svcPort.Name {
				targetPort = int(port.Port)
				found = true
				break
			}
		}
		if found {
			epsAddresses = subset.Addresses
			break
		}
	}
	if len(epsAddresses) == 0 {
		return nil, nil
	}
	var retBackends []*networkextensionv1.ListenerBackend
	for _, epAddr := range epsAddresses {
		retBackends = append(retBackends, &networkextensionv1.ListenerBackend{
			IP:     epAddr.IP,
			Port:   targetPort,
			Weight: weight,
		})
	}
	return retBackends, nil
}

func (rc *RuleConverter) getSubsetBackends(
	svc *k8scorev1.Service, subset *networkextensionv1.IngressSubset,
	epsBackends []*networkextensionv1.ListenerBackend) ([]*networkextensionv1.ListenerBackend, error) {
	labels := make(map[string]string)
	for k, v := range svc.Spec.Selector {
		labels[k] = v
	}
	for k, v := range subset.LabelSelector {
		labels[k] = v
	}
	pods, err := rc.getPodsByLabels(svc.GetNamespace(), labels)
	if err != nil {
		return nil, err
	}
	epsMap := make(map[string]*networkextensionv1.ListenerBackend)
	for _, epBackend := range epsBackends {
		epsMap[epBackend.IP] = epBackend
	}

	var retBackends []*networkextensionv1.ListenerBackend
	for _, pod := range pods {
		if len(pod.Status.PodIP) == 0 {
			continue
		}
		if backend, ok := epsMap[pod.Status.PodIP]; ok {
			retBackends = append(retBackends, &networkextensionv1.ListenerBackend{
				IP:     backend.IP,
				Port:   backend.Port,
				Weight: subset.GetWeight(),
			})
		}
	}
	return retBackends, nil
}

func (rc *RuleConverter) getNodePortBackends(
	svc *k8scorev1.Service, svcPort *k8scorev1.ServicePort) (
	[]*networkextensionv1.ListenerBackend, error) {

	if svcPort.NodePort <= 0 {
		blog.Warnf("get no node port of service %s/%s 's port %+v",
			svc.GetNamespace(), svc.GetName(), svcPort)
		return nil, nil
	}

	pods, err := rc.getPodsByLabels(svc.GetNamespace(), svc.Spec.Selector)
	if err != nil {
		return nil, err
	}

	var retBackends []*networkextensionv1.ListenerBackend
	for _, pod := range pods {
		if len(pod.Status.HostIP) == 0 {
			continue
		}
		retBackends = append(retBackends, &networkextensionv1.ListenerBackend{
			IP:     pod.Status.HostIP,
			Port:   int(svcPort.NodePort),
			Weight: networkextensionv1.DefaultWeight,
		})
	}
	return retBackends, nil
}

func (rc *RuleConverter) getPodsByLabels(ns string, labels map[string]string) ([]*k8scorev1.Pod, error) {
	podList := &k8scorev1.PodList{}
	err := rc.cli.List(context.TODO(), podList, client.MatchingLabels(labels), &client.ListOptions{Namespace: ns})
	if err != nil {
		blog.Errorf("list pod list failed by labels %+v and ns %s, err %s", labels, ns, err.Error())
		return nil, fmt.Errorf("list pod list failed by labels %+v and ns %s, err %s", labels, ns, err.Error())
	}
	var retPods []*k8scorev1.Pod
	for i := 0; i < len(podList.Items); i++ {
		retPods = append(retPods, &podList.Items[i])
	}
	return retPods, nil
}