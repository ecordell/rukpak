// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	v1alpha1 "github.com/ecordell/rukpak/pkg/apis/rukpak/clientset/versioned/typed/rukpak/v1alpha1"
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"
)

type FakeRukpakV1alpha1 struct {
	*testing.Fake
}

func (c *FakeRukpakV1alpha1) Bundles(namespace string) v1alpha1.BundleInterface {
	return &FakeBundles{c, namespace}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeRukpakV1alpha1) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}