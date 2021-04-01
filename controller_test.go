package main

import (
	"context"
	"fmt"
	"github.com/ecordell/rukpak/pkg/apis/rukpak/clientset/versioned/fake"
	"github.com/ecordell/rukpak/pkg/apis/rukpak/v1alpha1"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func TestMachine(t *testing.T) {
	initial := &v1alpha1.Bundle{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "test", Generation: 4}}
	rukpakclientset := fake.NewSimpleClientset(initial)
	machine := BundleMachine.WithContext(BundleMachineContext{client: rukpakclientset})
	machine.Send(BundleSyncEvent(initial))
	out, err := rukpakclientset.RukpakV1alpha1().Bundles("test").Get(context.TODO(), "test", metav1.GetOptions{})
	require.NoError(t, err)
	fmt.Println(out.Status.ObservedGeneration)
	require.Equal(t, int64(4), out.Status.ObservedGeneration)

	//interpreter :=  NewInterpreter(machine)
	//ctx := signals.Ctx()
	//require.NoError(t, interpreter.Start(1, ctx.Done()))
	//interpreter.Receive(BundleSyncEvent(initial))
}