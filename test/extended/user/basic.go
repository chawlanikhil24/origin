package user

import (
	"reflect"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	kubeauthorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	authorizationv1 "github.com/openshift/api/authorization/v1"
	userv1 "github.com/openshift/api/user/v1"
	projectv1typedclient "github.com/openshift/client-go/project/clientset/versioned/typed/project/v1"
	userv1typedclient "github.com/openshift/client-go/user/clientset/versioned/typed/user/v1"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Feature:UserAPI]", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLI("user-api", exutil.KubeConfigPath())

	g.It("users can manipulate groups", func() {
		t := g.GinkgoT()

		clusterAdminUserClient := oc.AdminUserClient().UserV1()

		valerieName := oc.CreateUser("valerie-").Name

		g.By("make sure we don't get back system groups", func() {
			// make sure we don't get back system groups
			userValerie, err := clusterAdminUserClient.Users().Get(valerieName, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(userValerie.Groups) != 0 {
				t.Errorf("unexpected groups: %v", userValerie.Groups)
			}
		})

		g.By("make sure that user/~ returns groups for unbacked users", func() {
			// make sure that user/~ returns groups for unbacked users
			expectedClusterAdminGroups := []string{"system:authenticated", "system:masters"}
			clusterAdminUser, err := clusterAdminUserClient.Users().Get("~", metav1.GetOptions{})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(clusterAdminUser.Groups, expectedClusterAdminGroups) {
				t.Errorf("expected %v, got %v", expectedClusterAdminGroups, clusterAdminUser.Groups)
			}
		})

		theGroup := &userv1.Group{}
		theGroup.Name = "theGroup-" + oc.Namespace()
		theGroup.Users = append(theGroup.Users, valerieName)
		_, err := clusterAdminUserClient.Groups().Create(theGroup)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		oc.AddResourceToDelete(userv1.GroupVersion.WithResource("groups"), theGroup)

		g.By("make sure that user/~ returns system groups for backed users when it merges", func() {
			// make sure that user/~ returns system groups for backed users when it merges
			expectedValerieGroups := []string{"system:authenticated", "system:authenticated:oauth"}
			valerieConfig := oc.GetClientConfigForUser(valerieName)
			secondValerie, err := userv1typedclient.NewForConfigOrDie(valerieConfig).Users().Get("~", metav1.GetOptions{})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(secondValerie.Groups, expectedValerieGroups) {
				t.Errorf("expected %v, got %v", expectedValerieGroups, secondValerie.Groups)
			}
		})

		g.By("confirm no access to the project", func() {
			// separate client here to avoid bad caching
			valerieConfig := oc.GetClientConfigForUser(valerieName)
			_, err = projectv1typedclient.NewForConfigOrDie(valerieConfig).Projects().Get(oc.Namespace(), metav1.GetOptions{})
			if err == nil {
				t.Fatalf("expected error")
			}
		})

		g.By("adding the binding", func() {
			roleBinding := &authorizationv1.RoleBinding{}
			roleBinding.Name = "admins"
			roleBinding.RoleRef.Name = "admin"
			roleBinding.Subjects = []corev1.ObjectReference{
				{Kind: "Group", Name: theGroup.Name},
			}
			_, err = oc.AdminAuthorizationClient().AuthorizationV1().RoleBindings(oc.Namespace()).Create(roleBinding)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			err = oc.WaitForAccessAllowed(&kubeauthorizationv1.SelfSubjectAccessReview{
				Spec: kubeauthorizationv1.SelfSubjectAccessReviewSpec{
					ResourceAttributes: &kubeauthorizationv1.ResourceAttributes{
						Namespace: oc.Namespace(),
						Verb:      "get",
						Group:     "",
						Resource:  "pods",
					},
				},
			}, valerieName)
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.By("make sure that user groups are respected for policy", func() {
			// make sure that user groups are respected for policy
			valerieConfig := oc.GetClientConfigForUser(valerieName)
			_, err = projectv1typedclient.NewForConfigOrDie(valerieConfig).Projects().Get(oc.Namespace(), metav1.GetOptions{})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	})

	g.It("groups should work", func() {
		t := g.GinkgoT()
		clusterAdminUserClient := oc.AdminUserClient().UserV1()

		victorName := oc.CreateUser("victor-").Name
		valerieName := oc.CreateUser("valerie-").Name
		valerieConfig := oc.GetClientConfigForUser(valerieName)

		g.By("creating the group")
		theGroup := &userv1.Group{}
		theGroup.Name = "thegroup-" + oc.Namespace()
		theGroup.Users = append(theGroup.Users, valerieName, victorName)
		_, err := clusterAdminUserClient.Groups().Create(theGroup)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		oc.AddResourceToDelete(userv1.GroupVersion.WithResource("groups"), theGroup)

		g.By("confirm no access to the project", func() {
			// separate client here to avoid bad caching
			valerieConfig := oc.GetClientConfigForUser(valerieName)
			_, err = projectv1typedclient.NewForConfigOrDie(valerieConfig).Projects().Get(oc.Namespace(), metav1.GetOptions{})
			if err == nil {
				t.Fatalf("expected error")
			}
		})

		g.By("adding the binding", func() {
			roleBinding := &authorizationv1.RoleBinding{}
			roleBinding.Name = "admins"
			roleBinding.RoleRef.Name = "admin"
			roleBinding.Subjects = []corev1.ObjectReference{
				{Kind: "Group", Name: theGroup.Name},
			}
			_, err = oc.AdminAuthorizationClient().AuthorizationV1().RoleBindings(oc.Namespace()).Create(roleBinding)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			err = oc.WaitForAccessAllowed(&kubeauthorizationv1.SelfSubjectAccessReview{
				Spec: kubeauthorizationv1.SelfSubjectAccessReviewSpec{
					ResourceAttributes: &kubeauthorizationv1.ResourceAttributes{
						Namespace: oc.Namespace(),
						Verb:      "list",
						Group:     "",
						Resource:  "pods",
					},
				},
			}, valerieName)
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.By("checking access", func() {
			// make sure that user groups are respected for policy
			_, err = projectv1typedclient.NewForConfigOrDie(valerieConfig).Projects().Get(oc.Namespace(), metav1.GetOptions{})
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			victorConfig := oc.GetClientConfigForUser(victorName)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			_, err = projectv1typedclient.NewForConfigOrDie(victorConfig).Projects().Get(oc.Namespace(), metav1.GetOptions{})
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	})
})
