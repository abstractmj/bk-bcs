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
	"sort"
	"strings"
	"time"

	gocache "github.com/patrickmn/go-cache"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/Tencent/bk-bcs/bcs-common/common/blog"
	networkextensionv1 "github.com/Tencent/bk-bcs/bcs-k8s/kubernetes/apis/networkextension/v1"
	"github.com/Tencent/bk-bcs/bcs-network/bcs-ingress-controller/internal/cloud"
)

// IngressConverter listener generator
type IngressConverter struct {
	defaultRegion    string
	cli              client.Client
	ingressValidater cloud.Validater
	lbClient         cloud.LoadBalance
	lbIDCache        *gocache.Cache
	lbNameCache      *gocache.Cache
}

// NewIngressConverter create ingress generator
func NewIngressConverter(
	region string, cli client.Client, ingressValidater cloud.Validater, lbClient cloud.LoadBalance) *IngressConverter {
	return &IngressConverter{
		defaultRegion:    region,
		cli:              cli,
		ingressValidater: ingressValidater,
		lbClient:         lbClient,
		lbIDCache:        gocache.New(60*time.Minute, 120*time.Minute),
		lbNameCache:      gocache.New(60*time.Minute, 120*time.Minute),
	}
}

func (g *IngressConverter) getLoadbalanceByID(regionIDPair string) (*cloud.LoadBalanceObject, error) {
	var lbObj *cloud.LoadBalanceObject
	var err error
	strs := strings.Split(regionIDPair, ":")
	if len(strs) == 1 {
		obj, ok := g.lbIDCache.Get(g.defaultRegion + ":" + strs[0])
		if ok {
			if lbObj, ok = obj.(*cloud.LoadBalanceObject); !ok {
				return nil, fmt.Errorf("get obj from lb id cache is not LoadBalanceObject")
			}
			return lbObj, nil
		}
		lbObj, err = g.lbClient.DescribeLoadBalancer(g.defaultRegion, strs[0], "")
		if err != nil {
			return nil, err
		}
	} else if len(strs) == 2 {
		obj, ok := g.lbIDCache.Get(regionIDPair)
		if ok {
			if lbObj, ok = obj.(*cloud.LoadBalanceObject); !ok {
				return nil, fmt.Errorf("get obj from lb id cache is not LoadBalanceObject")
			}
			return lbObj, nil
		}
		lbObj, err = g.lbClient.DescribeLoadBalancer(strs[0], strs[1], "")
		if err != nil {
			return nil, err
		}
	} else {
		blog.Warnf("lbid %s invalid", regionIDPair)
		return nil, fmt.Errorf("lbid %s invalid", regionIDPair)
	}
	g.lbIDCache.SetDefault(lbObj.Region+":"+lbObj.LbID, lbObj)
	g.lbNameCache.SetDefault(lbObj.Region+":"+lbObj.Name, lbObj)
	return lbObj, nil
}

func (g *IngressConverter) getLoadbalanceByName(regionNamePair string) (*cloud.LoadBalanceObject, error) {
	var lbObj *cloud.LoadBalanceObject
	var err error
	strs := strings.Split(regionNamePair, ":")
	if len(strs) == 1 {
		obj, ok := g.lbNameCache.Get(g.defaultRegion + ":" + strs[0])
		if ok {
			if lbObj, ok = obj.(*cloud.LoadBalanceObject); !ok {
				return nil, fmt.Errorf("get obj from lb name cache is not LoadBalanceObject")
			}
			return lbObj, nil
		}
		lbObj, err = g.lbClient.DescribeLoadBalancer(g.defaultRegion, "", strs[0])
		if err != nil {
			return nil, err
		}
	} else if len(strs) == 2 {
		obj, ok := g.lbNameCache.Get(regionNamePair)
		if ok {
			if lbObj, ok = obj.(*cloud.LoadBalanceObject); !ok {
				return nil, fmt.Errorf("get obj from lb id cache is not LoadBalanceObject")
			}
			return lbObj, nil
		}
		lbObj, err = g.lbClient.DescribeLoadBalancer(strs[0], "", strs[1])
		if err != nil {
			return nil, err
		}
	} else {
		blog.Warnf("lbname %s invalid", regionNamePair)
		return nil, fmt.Errorf("lbname %s invalid", regionNamePair)
	}
	g.lbIDCache.SetDefault(lbObj.Region+":"+lbObj.LbID, lbObj)
	g.lbNameCache.SetDefault(lbObj.Region+":"+lbObj.Name, lbObj)
	return lbObj, nil
}

// get ingress loadbalance objects
func (g *IngressConverter) getIngressLoadbalances(ingress *networkextensionv1.Ingress) (
	[]*cloud.LoadBalanceObject, error) {
	var lbs []*cloud.LoadBalanceObject
	lbIDStrs, idOk := ingress.Annotations[networkextensionv1.AnnotationKeyForLoadbalanceIDs]
	lbNameStrs, nameOk := ingress.Annotations[networkextensionv1.AnnotationKeyForLoadbalanceNames]
	if !idOk && !nameOk {
		blog.Warnf("ingress %+v is not associated with lb instance")
		return nil, nil
	}

	if idOk {
		lbIDs := strings.Split(lbIDStrs, ",")
		for _, regionIDPair := range lbIDs {
			lbObj, err := g.getLoadbalanceByID(regionIDPair)
			if err != nil {
				return nil, err
			}
			lbs = append(lbs, lbObj)
		}
	} else if nameOk {
		names := strings.Split(lbNameStrs, ",")
		for _, regionNamePair := range names {
			lbObj, err := g.getLoadbalanceByName(regionNamePair)
			if err != nil {
				return nil, err
			}
			lbs = append(lbs, lbObj)
		}
	}
	return lbs, nil
}

// ProcessUpdateIngress process newly added or updated ingress
func (g *IngressConverter) ProcessUpdateIngress(ingress *networkextensionv1.Ingress) error {
	isValid, errMsg := g.ingressValidater.IsIngressValid(ingress)
	if !isValid {
		blog.Errorf("ingress %+v ingress is invalid, err %s", ingress, errMsg)
		return fmt.Errorf("ingress %+v ingress is invalid, err %s", ingress, errMsg)
	}

	isValid, errMsg = checkConflictsInIngress(ingress)
	if !isValid {
		blog.Errorf("ingress %+v ingress has conflicts, err %s", ingress, errMsg)
		return fmt.Errorf("ingress %+v ingress has conflicts, err %s", ingress, errMsg)
	}

	lbObjs, err := g.getIngressLoadbalances(ingress)
	if err != nil {
		return err
	}

	for _, lbObj := range lbObjs {
		isConflict, err := g.checkConflicts(lbObj.LbID, ingress)
		if err != nil {
			return err
		}
		if isConflict {
			blog.Errorf("ingress %+v is conflict with existed listeners", ingress)
			return fmt.Errorf("ingress %+v is conflict with existed listeners", ingress)
		}
	}

	var generatedListeners []networkextensionv1.Listener
	var generatedSegListeners []networkextensionv1.Listener
	for _, rule := range ingress.Spec.Rules {
		ruleConverter := NewRuleConverter(g.cli, lbObjs, ingress.GetName(), ingress.GetNamespace(), &rule)
		listeners, err := ruleConverter.DoConvert()
		if err != nil {
			blog.Errorf("convert rule %+v failed, err %s", rule, err.Error())
			return fmt.Errorf("convert rule %+v failed, err %s", rule, err.Error())
		}
		generatedListeners = append(generatedListeners, listeners...)
	}
	for _, mapping := range ingress.Spec.PortMappings {
		mappingConverter := NewMappingConverter(g.cli, lbObjs, ingress.GetName(), ingress.GetNamespace(), &mapping)
		listeners, err := mappingConverter.DoConvert()
		if err != nil {
			blog.Errorf("convert mapping %+v failed, err %s", mapping, err.Error())
			return fmt.Errorf("convert mapping %+v failed, err %s", mapping, err.Error())
		}
		// if ignore segment, disable segment feature;
		// if segment length is not set or equals to 1, disable segment feature;
		if mapping.IgnoreSegment || mapping.SegmentLength == 0 || mapping.SegmentLength == 1 {
			generatedListeners = append(generatedListeners, listeners...)
		} else {
			generatedSegListeners = append(generatedSegListeners, listeners...)
		}
	}
	sort.Sort(networkextensionv1.ListenerSlice(generatedListeners))
	sort.Sort(networkextensionv1.ListenerSlice(generatedSegListeners))

	existedListeners, err := g.getListeners(ingress.GetName(), ingress.GetNamespace())
	if err != nil {
		return err
	}
	existedSegListeners, err := g.getSegmentListeners(ingress.GetName(), ingress.GetNamespace())
	if err != nil {
		return err
	}
	err = g.syncListeners(ingress.GetName(), ingress.GetNamespace(),
		existedListeners, generatedListeners, existedSegListeners, generatedSegListeners)
	if err != nil {
		blog.Errorf("syncListeners listener failed, err %s", err.Error())
		return fmt.Errorf("syncListeners listener failed, err %s", err.Error())
	}
	return nil
}

// ProcessDeleteIngress  process deleted ingress
func (g *IngressConverter) ProcessDeleteIngress(ingressName, ingressNamespace string) error {
	listener := &networkextensionv1.Listener{}
	selector, err := k8smetav1.LabelSelectorAsSelector(k8smetav1.SetAsLabelSelector(k8slabels.Set(map[string]string{
		ingressName:      networkextensionv1.LabelValueForIngressName,
		ingressNamespace: networkextensionv1.LabelValueForIngressNamespace,
	})))
	if err != nil {
		blog.Errorf("get selector for deleted ingress %s/%s failed, err %s",
			ingressName, ingressNamespace, err.Error())
		return fmt.Errorf("get selector for deleted ingress %s/%s failed, err %s",
			ingressName, ingressNamespace, err.Error())
	}
	err = g.cli.DeleteAllOf(context.TODO(), listener,
		&client.DeleteAllOfOptions{
			ListOptions: client.ListOptions{
				LabelSelector: selector,
				Namespace:     ingressNamespace,
			},
		})
	if err != nil {
		blog.Errorf("delete listener by label selector %s, err %s", selector.String(), err.Error())
		return fmt.Errorf("delete listener by label selector %s, err %s", selector.String(), err.Error())
	}
	return nil
}

func (g *IngressConverter) getListeners(ingressName, ingressNamespace string) (
	[]networkextensionv1.Listener, error) {
	existedListenerList := &networkextensionv1.ListenerList{}
	selector, err := k8smetav1.LabelSelectorAsSelector(k8smetav1.SetAsLabelSelector(k8slabels.Set(map[string]string{
		ingressName:      networkextensionv1.LabelValueForIngressName,
		ingressNamespace: networkextensionv1.LabelValueForIngressNamespace,
		networkextensionv1.LabelKeyForIsSegmentListener: networkextensionv1.LabelValueFalse,
	})))
	err = g.cli.List(context.TODO(), existedListenerList, &client.ListOptions{LabelSelector: selector})
	if err != nil {
		blog.Errorf("list listeners ingress %s/%s failed, err %s",
			ingressName, ingressNamespace, err.Error())
		return nil, fmt.Errorf("list listeners ingress %s/%s failed, err %s",
			ingressName, ingressNamespace, err.Error())
	}
	return existedListenerList.Items, nil
}

func (g *IngressConverter) getSegmentListeners(ingressName, ingressNamespace string) (
	[]networkextensionv1.Listener, error) {
	existedListenerList := &networkextensionv1.ListenerList{}
	selector, err := k8smetav1.LabelSelectorAsSelector(k8smetav1.SetAsLabelSelector(k8slabels.Set(map[string]string{
		ingressName:      networkextensionv1.LabelValueForIngressName,
		ingressNamespace: networkextensionv1.LabelValueForIngressNamespace,
		networkextensionv1.LabelKeyForIsSegmentListener: networkextensionv1.LabelValueTrue,
	})))
	err = g.cli.List(context.TODO(), existedListenerList, &client.ListOptions{LabelSelector: selector})
	if err != nil {
		blog.Errorf("list segment listeners ingress %s/%s failed, err %s",
			ingressName, ingressNamespace, err.Error())
		return nil, fmt.Errorf("list segment listeners ingress %s/%s failed, err %s",
			ingressName, ingressNamespace, err.Error())
	}
	return existedListenerList.Items, nil
}

func (g *IngressConverter) syncListeners(ingressName, ingressNamespace string,
	existedListeners, listeners []networkextensionv1.Listener,
	existedSegListeners, segListeners []networkextensionv1.Listener) error {

	adds, dels, olds, news := GetDiffListeners(existedListeners, listeners)
	sadds, sdels, solds, snews := GetDiffListeners(existedSegListeners, segListeners)
	adds = append(adds, sadds...)
	dels = append(dels, sdels...)
	olds = append(olds, solds...)
	news = append(news, snews...)
	for _, del := range dels {
		blog.V(3).Infof("[generator] delete listener %s/%s", del.GetNamespace(), del.GetName())
		err := g.cli.Delete(context.TODO(), &del, &client.DeleteOptions{})
		if err != nil {
			blog.Errorf("delete listener %+v failed, err %s", del, err.Error())
			return fmt.Errorf("delete listener %+v failed, err %s", del, err.Error())
		}
	}
	for _, add := range adds {
		blog.V(3).Infof("[generator] create listener %s/%s", add.GetNamespace(), add.GetName())
		err := g.cli.Create(context.TODO(), &add, &client.CreateOptions{})
		if err != nil {
			blog.Errorf("create listener %+v failed, err %s", add, err.Error())
			return fmt.Errorf("create listener %+v failed, err %s", add, err.Error())
		}
	}
	for index, new := range news {
		blog.V(3).Infof("[generator] update listener %s/%s", new.GetNamespace(), new.GetName())
		new.ResourceVersion = olds[index].ResourceVersion
		err := g.cli.Update(context.TODO(), &new, &client.UpdateOptions{})
		if err != nil {
			blog.Errorf("update listener %+v failed, err %s", new, err.Error())
			return fmt.Errorf("update listener %+v failed, err %s", new, err.Error())
		}
	}
	return nil
}
