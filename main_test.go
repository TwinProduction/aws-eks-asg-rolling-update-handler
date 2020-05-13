package main

import (
	"github.com/TwinProduction/aws-eks-asg-rolling-update-handler/cloudtest"
	"github.com/TwinProduction/aws-eks-asg-rolling-update-handler/k8s"
	"github.com/TwinProduction/aws-eks-asg-rolling-update-handler/k8stest"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
	v1 "k8s.io/api/core/v1"
	"testing"
)

func TestSeparateOutdatedFromUpdatedInstancesUsingLaunchConfiguration_whenInstanceIsOutdated(t *testing.T) {
	instance := cloudtest.CreateTestAutoScalingInstance("instance", "v1", nil, "InService")
	outdated, updated, err := SeparateOutdatedFromUpdatedInstancesUsingLaunchConfiguration(aws.String("v2"), []*autoscaling.Instance{instance})
	if err != nil {
		t.Fatal("Shouldn't have returned an error, but returned", err)
	}
	if len(outdated) != 1 || len(updated) != 0 {
		t.Error("Instance should've been outdated")
	}
}

func TestSeparateOutdatedFromUpdatedInstancesUsingLaunchConfiguration_whenInstanceIsUpdated(t *testing.T) {
	instance := cloudtest.CreateTestAutoScalingInstance("instance", "v1", nil, "InService")
	outdated, updated, err := SeparateOutdatedFromUpdatedInstancesUsingLaunchConfiguration(aws.String("v1"), []*autoscaling.Instance{instance})
	if err != nil {
		t.Fatal("Shouldn't have returned an error, but returned", err)
	}
	if len(outdated) != 0 || len(updated) != 1 {
		t.Error("Instance should've been updated")
	}
}

func TestSeparateOutdatedFromUpdatedInstancesUsingLaunchConfiguration_whenOneInstanceIsUpdatedAndTwoInstancesAreOutdated(t *testing.T) {
	firstInstance := cloudtest.CreateTestAutoScalingInstance("old-1", "v1", nil, "InService")
	secondInstance := cloudtest.CreateTestAutoScalingInstance("old-2", "v1", nil, "InService")
	thirdInstance := cloudtest.CreateTestAutoScalingInstance("new", "v2", nil, "InService")
	outdated, updated, err := SeparateOutdatedFromUpdatedInstancesUsingLaunchConfiguration(aws.String("v2"), []*autoscaling.Instance{firstInstance, secondInstance, thirdInstance})
	if err != nil {
		t.Fatal("Shouldn't have returned an error, but returned", err)
	}
	if len(outdated) != 2 {
		t.Error("2 instances should've been outdated")
	}
	if len(updated) != 1 {
		t.Error("1 instance should've been outdated")
	}
}

func TestSeparateOutdatedFromUpdatedInstancesUsingLaunchTemplate_whenInstanceIsOutdated(t *testing.T) {
	outdatedLaunchTemplate := &autoscaling.LaunchTemplateSpecification{
		LaunchTemplateId:   aws.String("id"),
		LaunchTemplateName: aws.String("name"),
		Version:            aws.String("v1"),
	}
	updatedLaunchTemplate := &autoscaling.LaunchTemplateSpecification{
		LaunchTemplateId:   aws.String("id"),
		LaunchTemplateName: aws.String("name"),
		Version:            aws.String("v2"),
	}
	updatedEc2LaunchTemplate := &ec2.LaunchTemplate{
		DefaultVersionNumber: aws.Int64(1),
		LatestVersionNumber:  aws.Int64(10),
		LaunchTemplateId:     updatedLaunchTemplate.LaunchTemplateId,
		LaunchTemplateName:   updatedLaunchTemplate.LaunchTemplateName,
	}
	instance := cloudtest.CreateTestAutoScalingInstance("instance", "", outdatedLaunchTemplate, "InService")
	outdated, updated, err := SeparateOutdatedFromUpdatedInstancesUsingLaunchTemplate(updatedLaunchTemplate, []*autoscaling.Instance{instance}, cloudtest.NewMockEC2Service([]*ec2.LaunchTemplate{updatedEc2LaunchTemplate}))
	if err != nil {
		t.Fatal("Shouldn't have returned an error, but returned:", err)
	}
	if len(outdated) != 1 || len(updated) != 0 {
		t.Error("Instance should've been outdated")
	}
}

func TestSeparateOutdatedFromUpdatedInstancesUsingLaunchTemplate_whenInstanceIsUpdated(t *testing.T) {
	updatedLaunchTemplate := &autoscaling.LaunchTemplateSpecification{
		LaunchTemplateId:   aws.String("id"),
		LaunchTemplateName: aws.String("name"),
		Version:            aws.String("v1"),
	}
	updatedEc2LaunchTemplate := &ec2.LaunchTemplate{
		DefaultVersionNumber: aws.Int64(1),
		LatestVersionNumber:  aws.Int64(10),
		LaunchTemplateId:     updatedLaunchTemplate.LaunchTemplateId,
		LaunchTemplateName:   updatedLaunchTemplate.LaunchTemplateName,
	}
	instance := cloudtest.CreateTestAutoScalingInstance("instance", "", updatedLaunchTemplate, "InService")
	outdated, updated, err := SeparateOutdatedFromUpdatedInstancesUsingLaunchTemplate(updatedLaunchTemplate, []*autoscaling.Instance{instance}, cloudtest.NewMockEC2Service([]*ec2.LaunchTemplate{updatedEc2LaunchTemplate}))
	if err != nil {
		t.Fatal("Shouldn't have returned an error, but returned:", err)
	}
	if len(outdated) != 0 || len(updated) != 1 {
		t.Error("Instance should've been updated")
	}
}

func TestSeparateOutdatedFromUpdatedInstances_withLaunchConfigurationWhenOneInstanceIsUpdatedAndTwoInstancesAreOutdated(t *testing.T) {
	firstInstance := cloudtest.CreateTestAutoScalingInstance("old-1", "v1", nil, "InService")
	secondInstance := cloudtest.CreateTestAutoScalingInstance("old-2", "v1", nil, "InService")
	thirdInstance := cloudtest.CreateTestAutoScalingInstance("new", "v2", nil, "InService")

	asg := cloudtest.CreateTestAutoScalingGroup("asg", "v2", nil, []*autoscaling.Instance{firstInstance, secondInstance, thirdInstance}, 0, 999)

	outdated, updated, err := SeparateOutdatedFromUpdatedInstances(asg, nil)
	if err != nil {
		t.Fatal("Shouldn't have returned an error, but returned", err)
	}
	if len(outdated) != 2 {
		t.Error("2 instances should've been outdated")
	}
	if len(updated) != 1 {
		t.Error("1 instance should've been outdated")
	}
}

func TestHandleRollingUpgrade(t *testing.T) {
	oldInstance := cloudtest.CreateTestAutoScalingInstance("old-1", "v1", nil, "InService")
	asg := cloudtest.CreateTestAutoScalingGroup("asg", "v2", nil, []*autoscaling.Instance{oldInstance}, 0, 999)

	oldNode := k8stest.CreateTestNode(aws.StringValue(oldInstance.InstanceId), "1000m", "1000Mi")
	oldNodePod := k8stest.CreateTestPod("old-pod-1", oldNode.Name, "100m", "100Mi", false)

	mockKubernetesClient := k8stest.NewMockKubernetesClient([]v1.Node{oldNode}, []v1.Pod{oldNodePod})
	mockEc2Service := cloudtest.NewMockEC2Service(nil)
	mockAutoScalingService := cloudtest.NewMockAutoScalingService([]*autoscaling.Group{asg})

	// First run (Node rollout process gets marked as started)
	HandleRollingUpgrade(mockKubernetesClient, mockEc2Service, mockAutoScalingService, []*autoscaling.Group{asg})
	if mockKubernetesClient.Counter["UpdateNode"] != 1 {
		t.Error("Node should've been annotated, meaning that UpdateNode should've been called once")
	}
	oldNode = mockKubernetesClient.Nodes[aws.StringValue(oldInstance.InstanceId)]
	if _, ok := oldNode.GetAnnotations()[k8s.RollingUpdateStartedTimestampAnnotationKey]; !ok {
		t.Error("Node should've been annotated with", k8s.RollingUpdateStartedTimestampAnnotationKey)
	}
	if _, ok := oldNode.GetAnnotations()[k8s.RollingUpdateDrainedTimestampAnnotationKey]; ok {
		t.Error("Node shouldn't have been drained yet, therefore shouldn't have been annotated with", k8s.RollingUpdateDrainedTimestampAnnotationKey)
	}
	if _, ok := oldNode.GetAnnotations()[k8s.RollingUpdateTerminatedTimestampAnnotationKey]; ok {
		t.Error("Node shouldn't have been terminated yet, therefore shouldn't have been annotated with", k8s.RollingUpdateTerminatedTimestampAnnotationKey)
	}

	// Second run (ASG's desired capacity gets increased)
	HandleRollingUpgrade(mockKubernetesClient, mockEc2Service, mockAutoScalingService, []*autoscaling.Group{asg})
	if mockAutoScalingService.Counter["SetDesiredCapacity"] != 1 {
		t.Error("ASG should've been increased because there's no updated nodes yet")
	}
	asg = mockAutoScalingService.AutoScalingGroups[aws.StringValue(asg.AutoScalingGroupName)]
	if aws.Int64Value(asg.DesiredCapacity) != 2 {
		t.Error("The desired capacity of the ASG should've been increased to 2")
	}
	oldNode = mockKubernetesClient.Nodes[aws.StringValue(oldInstance.InstanceId)]
	if _, ok := oldNode.GetAnnotations()[k8s.RollingUpdateDrainedTimestampAnnotationKey]; ok {
		t.Error("Node shouldn't have been drained yet, therefore shouldn't have been annotated with", k8s.RollingUpdateDrainedTimestampAnnotationKey)
	}

	// Third run (Nothing changed)
	HandleRollingUpgrade(mockKubernetesClient, mockEc2Service, mockAutoScalingService, []*autoscaling.Group{asg})
	if mockAutoScalingService.Counter["SetDesiredCapacity"] != 1 {
		t.Error("Desired capacity shouldn't have been updated")
	}
	asg = mockAutoScalingService.AutoScalingGroups[aws.StringValue(asg.AutoScalingGroupName)]
	if aws.Int64Value(asg.DesiredCapacity) != 2 {
		t.Error("The desired capacity of the ASG should've stayed at 2")
	}
	oldNode = mockKubernetesClient.Nodes[aws.StringValue(oldInstance.InstanceId)]
	if _, ok := oldNode.GetAnnotations()[k8s.RollingUpdateDrainedTimestampAnnotationKey]; ok {
		t.Error("Node shouldn't have been drained yet, therefore shouldn't have been annotated with", k8s.RollingUpdateDrainedTimestampAnnotationKey)
	}

	// Fourth run (new instance has been registered to ASG, but is pending)
	newInstance := cloudtest.CreateTestAutoScalingInstance("new-1", "v2", nil, "Pending")
	asg.Instances = append(asg.Instances, newInstance)
	HandleRollingUpgrade(mockKubernetesClient, mockEc2Service, mockAutoScalingService, []*autoscaling.Group{asg})
	if mockAutoScalingService.Counter["SetDesiredCapacity"] != 1 {
		t.Error("Desired capacity shouldn't have been updated")
	}
	oldNode = mockKubernetesClient.Nodes[aws.StringValue(oldInstance.InstanceId)]
	if _, ok := oldNode.GetAnnotations()[k8s.RollingUpdateDrainedTimestampAnnotationKey]; ok {
		t.Error("Node shouldn't have been drained yet, therefore shouldn't have been annotated with", k8s.RollingUpdateDrainedTimestampAnnotationKey)
	}

	// Fifth run (new instance is now InService, but node has still not joined cluster (GetNodeByHostName should return not found))
	newInstance.SetLifecycleState("InService")
	HandleRollingUpgrade(mockKubernetesClient, mockEc2Service, mockAutoScalingService, []*autoscaling.Group{asg})
	oldNode = mockKubernetesClient.Nodes[aws.StringValue(oldInstance.InstanceId)]
	if _, ok := oldNode.GetAnnotations()[k8s.RollingUpdateDrainedTimestampAnnotationKey]; ok {
		t.Error("Node shouldn't have been drained yet, therefore shouldn't have been annotated with", k8s.RollingUpdateDrainedTimestampAnnotationKey)
	}

	// Sixth run (new instance has joined the cluster, but Kubelet isn't ready to accept pods yet)
	newNode := k8stest.CreateTestNode(aws.StringValue(newInstance.InstanceId), "1000m", "1000Mi")
	newNode.Status.Conditions = []v1.NodeCondition{{Type: v1.NodeReady, Status: v1.ConditionFalse}}
	mockKubernetesClient.Nodes[newNode.Name] = newNode
	HandleRollingUpgrade(mockKubernetesClient, mockEc2Service, mockAutoScalingService, []*autoscaling.Group{asg})
	oldNode = mockKubernetesClient.Nodes[aws.StringValue(oldInstance.InstanceId)]
	if _, ok := oldNode.GetAnnotations()[k8s.RollingUpdateDrainedTimestampAnnotationKey]; ok {
		t.Error("Node shouldn't have been drained yet, therefore shouldn't have been annotated with", k8s.RollingUpdateDrainedTimestampAnnotationKey)
	}

	// Seventh run (Kubelet is ready to accept new pods. Old node gets drained and terminated)
	newNode = mockKubernetesClient.Nodes[newNode.Name]
	newNode.Status.Conditions = []v1.NodeCondition{{Type: v1.NodeReady, Status: v1.ConditionTrue}}
	mockKubernetesClient.Nodes[newNode.Name] = newNode
	HandleRollingUpgrade(mockKubernetesClient, mockEc2Service, mockAutoScalingService, []*autoscaling.Group{asg})
	oldNode = mockKubernetesClient.Nodes[aws.StringValue(oldInstance.InstanceId)]
	if _, ok := oldNode.GetAnnotations()[k8s.RollingUpdateDrainedTimestampAnnotationKey]; !ok {
		t.Error("Node should've been drained")
	}
	if _, ok := oldNode.GetAnnotations()[k8s.RollingUpdateTerminatedTimestampAnnotationKey]; !ok {
		t.Error("Node should've been terminated")
	}
}

func TestHandleRollingUpgrade_withEnoughPodsToRequireTwoNewNodes(t *testing.T) {
	oldInstance := cloudtest.CreateTestAutoScalingInstance("old-1", "v1", nil, "InService")
	asg := cloudtest.CreateTestAutoScalingGroup("asg", "v2", nil, []*autoscaling.Instance{oldInstance}, 0, 999)

	oldNode := k8stest.CreateTestNode(aws.StringValue(oldInstance.InstanceId), "1000m", "1000Mi")
	oldNodeFirstPod := k8stest.CreateTestPod("old-pod-1", oldNode.Name, "300m", "300Mi", false)
	oldNodeSecondPod := k8stest.CreateTestPod("old-pod-2", oldNode.Name, "300m", "300Mi", false)
	oldNodeThirdPod := k8stest.CreateTestPod("old-pod-3", oldNode.Name, "300m", "300Mi", false)
	oldNodeFourthPod := k8stest.CreateTestPod("old-pod-4", oldNode.Name, "300m", "300Mi", false)

	mockKubernetesClient := k8stest.NewMockKubernetesClient([]v1.Node{oldNode}, []v1.Pod{oldNodeFirstPod, oldNodeSecondPod, oldNodeThirdPod, oldNodeFourthPod})
	mockEc2Service := cloudtest.NewMockEC2Service(nil)
	mockAutoScalingService := cloudtest.NewMockAutoScalingService([]*autoscaling.Group{asg})

	// First run (Node rollout process gets marked as started)
	HandleRollingUpgrade(mockKubernetesClient, mockEc2Service, mockAutoScalingService, []*autoscaling.Group{asg})
	if mockKubernetesClient.Counter["UpdateNode"] != 1 {
		t.Error("Node should've been annotated, meaning that UpdateNode should've been called once")
	}
	oldNode = mockKubernetesClient.Nodes[aws.StringValue(oldInstance.InstanceId)]
	if _, ok := oldNode.GetAnnotations()[k8s.RollingUpdateStartedTimestampAnnotationKey]; !ok {
		t.Error("Node should've been annotated with", k8s.RollingUpdateStartedTimestampAnnotationKey)
	}
	if _, ok := oldNode.GetAnnotations()[k8s.RollingUpdateDrainedTimestampAnnotationKey]; ok {
		t.Error("Node shouldn't have been drained yet, therefore shouldn't have been annotated with", k8s.RollingUpdateDrainedTimestampAnnotationKey)
	}
	if _, ok := oldNode.GetAnnotations()[k8s.RollingUpdateTerminatedTimestampAnnotationKey]; ok {
		t.Error("Node shouldn't have been terminated yet, therefore shouldn't have been annotated with", k8s.RollingUpdateTerminatedTimestampAnnotationKey)
	}

	// Second run (ASG's desired capacity gets increased)
	HandleRollingUpgrade(mockKubernetesClient, mockEc2Service, mockAutoScalingService, []*autoscaling.Group{asg})
	if mockAutoScalingService.Counter["SetDesiredCapacity"] != 1 {
		t.Error("ASG should've been increased because there's no updated nodes yet")
	}
	asg = mockAutoScalingService.AutoScalingGroups[aws.StringValue(asg.AutoScalingGroupName)]
	if aws.Int64Value(asg.DesiredCapacity) != 2 {
		t.Error("The desired capacity of the ASG should've been increased to 2")
	}
	oldNode = mockKubernetesClient.Nodes[aws.StringValue(oldInstance.InstanceId)]
	if _, ok := oldNode.GetAnnotations()[k8s.RollingUpdateDrainedTimestampAnnotationKey]; ok {
		t.Error("Node shouldn't have been drained yet, therefore shouldn't have been annotated with", k8s.RollingUpdateDrainedTimestampAnnotationKey)
	}

	// Third run (Nothing changed)
	HandleRollingUpgrade(mockKubernetesClient, mockEc2Service, mockAutoScalingService, []*autoscaling.Group{asg})
	if mockAutoScalingService.Counter["SetDesiredCapacity"] != 1 {
		t.Error("Desired capacity shouldn't have been updated")
	}
	asg = mockAutoScalingService.AutoScalingGroups[aws.StringValue(asg.AutoScalingGroupName)]
	if aws.Int64Value(asg.DesiredCapacity) != 2 {
		t.Error("The desired capacity of the ASG should've stayed at 2")
	}
	oldNode = mockKubernetesClient.Nodes[aws.StringValue(oldInstance.InstanceId)]
	if _, ok := oldNode.GetAnnotations()[k8s.RollingUpdateDrainedTimestampAnnotationKey]; ok {
		t.Error("Node shouldn't have been drained yet, therefore shouldn't have been annotated with", k8s.RollingUpdateDrainedTimestampAnnotationKey)
	}

	// Fourth run (new instance has been registered to ASG, but is pending)
	newInstance := cloudtest.CreateTestAutoScalingInstance("new-1", "v2", nil, "Pending")
	asg.Instances = append(asg.Instances, newInstance)
	HandleRollingUpgrade(mockKubernetesClient, mockEc2Service, mockAutoScalingService, []*autoscaling.Group{asg})
	if mockAutoScalingService.Counter["SetDesiredCapacity"] != 1 {
		t.Error("Desired capacity shouldn't have been updated")
	}
	oldNode = mockKubernetesClient.Nodes[aws.StringValue(oldInstance.InstanceId)]
	if _, ok := oldNode.GetAnnotations()[k8s.RollingUpdateDrainedTimestampAnnotationKey]; ok {
		t.Error("Node shouldn't have been drained yet, therefore shouldn't have been annotated with", k8s.RollingUpdateDrainedTimestampAnnotationKey)
	}

	// Fifth run (new instance is now InService, but node has still not joined cluster (GetNodeByHostName should return not found))
	newInstance.SetLifecycleState("InService")
	HandleRollingUpgrade(mockKubernetesClient, mockEc2Service, mockAutoScalingService, []*autoscaling.Group{asg})
	oldNode = mockKubernetesClient.Nodes[aws.StringValue(oldInstance.InstanceId)]
	if _, ok := oldNode.GetAnnotations()[k8s.RollingUpdateDrainedTimestampAnnotationKey]; ok {
		t.Error("Node shouldn't have been drained yet, therefore shouldn't have been annotated with", k8s.RollingUpdateDrainedTimestampAnnotationKey)
	}

	// Sixth run (new instance has joined the cluster, but Kubelet isn't ready to accept pods yet)
	newNode := k8stest.CreateTestNode(aws.StringValue(newInstance.InstanceId), "1000m", "1000Mi")
	newNode.Status.Conditions = []v1.NodeCondition{{Type: v1.NodeReady, Status: v1.ConditionFalse}}
	mockKubernetesClient.Nodes[newNode.Name] = newNode
	HandleRollingUpgrade(mockKubernetesClient, mockEc2Service, mockAutoScalingService, []*autoscaling.Group{asg})
	oldNode = mockKubernetesClient.Nodes[aws.StringValue(oldInstance.InstanceId)]
	if _, ok := oldNode.GetAnnotations()[k8s.RollingUpdateDrainedTimestampAnnotationKey]; ok {
		t.Error("Node shouldn't have been drained yet, therefore shouldn't have been annotated with", k8s.RollingUpdateDrainedTimestampAnnotationKey)
	}

	// Seventh run (Kubelet is ready to accept new pods)
	newNode = mockKubernetesClient.Nodes[newNode.Name]
	newNode.Status.Conditions = []v1.NodeCondition{{Type: v1.NodeReady, Status: v1.ConditionTrue}}
	mockKubernetesClient.Nodes[newNode.Name] = newNode
	HandleRollingUpgrade(mockKubernetesClient, mockEc2Service, mockAutoScalingService, []*autoscaling.Group{asg})
	oldNode = mockKubernetesClient.Nodes[aws.StringValue(oldInstance.InstanceId)]
	if _, ok := oldNode.GetAnnotations()[k8s.RollingUpdateDrainedTimestampAnnotationKey]; ok {
		t.Error("Node shouldn't have been drained yet, therefore shouldn't have been annotated with", k8s.RollingUpdateDrainedTimestampAnnotationKey)
	}

	// Eight run (ASG's desired capacity gets increased)
	HandleRollingUpgrade(mockKubernetesClient, mockEc2Service, mockAutoScalingService, []*autoscaling.Group{asg})
	if mockAutoScalingService.Counter["SetDesiredCapacity"] != 2 {
		t.Error("ASG should've been increased again")
	}
	asg = mockAutoScalingService.AutoScalingGroups[aws.StringValue(asg.AutoScalingGroupName)]
	if aws.Int64Value(asg.DesiredCapacity) != 3 {
		t.Error("The desired capacity of the ASG should've been increased to 3")
	}
	oldNode = mockKubernetesClient.Nodes[aws.StringValue(oldInstance.InstanceId)]
	if _, ok := oldNode.GetAnnotations()[k8s.RollingUpdateDrainedTimestampAnnotationKey]; ok {
		t.Error("Node shouldn't have been drained yet, therefore shouldn't have been annotated with", k8s.RollingUpdateDrainedTimestampAnnotationKey)
	}

	// Ninth run (fast-forward new instance, node and kubelet ready to accept. Old node gets drained and terminated)
	newSecondInstance := cloudtest.CreateTestAutoScalingInstance("new-2", "v2", nil, "InService")
	asg.Instances = append(asg.Instances, newSecondInstance)
	newSecondNode := k8stest.CreateTestNode(aws.StringValue(newSecondInstance.InstanceId), "1000m", "1000Mi")
	newSecondNode.Status.Conditions = []v1.NodeCondition{{Type: v1.NodeReady, Status: v1.ConditionTrue}}
	mockKubernetesClient.Nodes[newSecondNode.Name] = newSecondNode
	HandleRollingUpgrade(mockKubernetesClient, mockEc2Service, mockAutoScalingService, []*autoscaling.Group{asg})
	oldNode = mockKubernetesClient.Nodes[aws.StringValue(oldInstance.InstanceId)]
	if _, ok := oldNode.GetAnnotations()[k8s.RollingUpdateDrainedTimestampAnnotationKey]; !ok {
		t.Error("Node should've been drained")
	}
	if _, ok := oldNode.GetAnnotations()[k8s.RollingUpdateTerminatedTimestampAnnotationKey]; !ok {
		t.Error("Node should've been terminated")
	}
}

func TestHandleRollingUpgrade_whenDesiredCapacityIsMaxCapacityBeforeRollingUpgrade(t *testing.T) {
	oldInstance := cloudtest.CreateTestAutoScalingInstance("old-1", "v1", nil, "InService")
	asg := cloudtest.CreateTestAutoScalingGroup("asg", "v2", nil, []*autoscaling.Instance{oldInstance}, 0, 1)

	oldNode := k8stest.CreateTestNode(aws.StringValue(oldInstance.InstanceId), "1000m", "1000Mi")
	oldNodePod := k8stest.CreateTestPod("old-pod-1", oldNode.Name, "100m", "100Mi", false)

	mockKubernetesClient := k8stest.NewMockKubernetesClient([]v1.Node{oldNode}, []v1.Pod{oldNodePod})
	mockEc2Service := cloudtest.NewMockEC2Service(nil)
	mockAutoScalingService := cloudtest.NewMockAutoScalingService([]*autoscaling.Group{asg})

	// First run (Node rollout process gets marked as started)
	HandleRollingUpgrade(mockKubernetesClient, mockEc2Service, mockAutoScalingService, []*autoscaling.Group{asg})
	if mockKubernetesClient.Counter["UpdateNode"] != 1 {
		t.Error("Node should've been annotated, meaning that UpdateNode should've been called once")
	}
	oldNode = mockKubernetesClient.Nodes[aws.StringValue(oldInstance.InstanceId)]
	if _, ok := oldNode.GetAnnotations()[k8s.RollingUpdateStartedTimestampAnnotationKey]; !ok {
		t.Error("Node should've been annotated with", k8s.RollingUpdateStartedTimestampAnnotationKey)
	}
	if _, ok := oldNode.GetAnnotations()[k8s.RollingUpdateDrainedTimestampAnnotationKey]; ok {
		t.Error("Node shouldn't have been drained yet, therefore shouldn't have been annotated with", k8s.RollingUpdateDrainedTimestampAnnotationKey)
	}
	if _, ok := oldNode.GetAnnotations()[k8s.RollingUpdateTerminatedTimestampAnnotationKey]; ok {
		t.Error("Node shouldn't have been terminated yet, therefore shouldn't have been annotated with", k8s.RollingUpdateTerminatedTimestampAnnotationKey)
	}

	// Second run (ASG's desired capacity gets increased)
	HandleRollingUpgrade(mockKubernetesClient, mockEc2Service, mockAutoScalingService, []*autoscaling.Group{asg})
	if mockAutoScalingService.Counter["SetDesiredCapacity"] != 1 {
		t.Error("ASG should've been increased because there's no updated nodes yet")
	}
	asg = mockAutoScalingService.AutoScalingGroups[aws.StringValue(asg.AutoScalingGroupName)]
	if aws.Int64Value(asg.DesiredCapacity) != 2 {
		t.Error("The desired capacity of the ASG should've been increased to 2")
	}
	oldNode = mockKubernetesClient.Nodes[aws.StringValue(oldInstance.InstanceId)]
	if _, ok := oldNode.GetAnnotations()[k8s.RollingUpdateDrainedTimestampAnnotationKey]; ok {
		t.Error("Node shouldn't have been drained yet, therefore shouldn't have been annotated with", k8s.RollingUpdateDrainedTimestampAnnotationKey)
	}

	// Third run (Nothing changed)
	HandleRollingUpgrade(mockKubernetesClient, mockEc2Service, mockAutoScalingService, []*autoscaling.Group{asg})
	if mockAutoScalingService.Counter["SetDesiredCapacity"] != 1 {
		t.Error("Desired capacity shouldn't have been updated")
	}
	asg = mockAutoScalingService.AutoScalingGroups[aws.StringValue(asg.AutoScalingGroupName)]
	if aws.Int64Value(asg.DesiredCapacity) != 2 {
		t.Error("The desired capacity of the ASG should've stayed at 2")
	}
	oldNode = mockKubernetesClient.Nodes[aws.StringValue(oldInstance.InstanceId)]
	if _, ok := oldNode.GetAnnotations()[k8s.RollingUpdateDrainedTimestampAnnotationKey]; ok {
		t.Error("Node shouldn't have been drained yet, therefore shouldn't have been annotated with", k8s.RollingUpdateDrainedTimestampAnnotationKey)
	}

	// Fourth run (new instance has been registered to ASG, but is pending)
	newInstance := cloudtest.CreateTestAutoScalingInstance("new-1", "v2", nil, "Pending")
	asg.Instances = append(asg.Instances, newInstance)
	HandleRollingUpgrade(mockKubernetesClient, mockEc2Service, mockAutoScalingService, []*autoscaling.Group{asg})
	if mockAutoScalingService.Counter["SetDesiredCapacity"] != 1 {
		t.Error("Desired capacity shouldn't have been updated")
	}
	oldNode = mockKubernetesClient.Nodes[aws.StringValue(oldInstance.InstanceId)]
	if _, ok := oldNode.GetAnnotations()[k8s.RollingUpdateDrainedTimestampAnnotationKey]; ok {
		t.Error("Node shouldn't have been drained yet, therefore shouldn't have been annotated with", k8s.RollingUpdateDrainedTimestampAnnotationKey)
	}

	// Fifth run (new instance is now InService, but node has still not joined cluster (GetNodeByHostName should return not found))
	newInstance.SetLifecycleState("InService")
	HandleRollingUpgrade(mockKubernetesClient, mockEc2Service, mockAutoScalingService, []*autoscaling.Group{asg})
	oldNode = mockKubernetesClient.Nodes[aws.StringValue(oldInstance.InstanceId)]
	if _, ok := oldNode.GetAnnotations()[k8s.RollingUpdateDrainedTimestampAnnotationKey]; ok {
		t.Error("Node shouldn't have been drained yet, therefore shouldn't have been annotated with", k8s.RollingUpdateDrainedTimestampAnnotationKey)
	}

	// Sixth run (new instance has joined the cluster, but Kubelet isn't ready to accept pods yet)
	newNode := k8stest.CreateTestNode(aws.StringValue(newInstance.InstanceId), "1000m", "1000Mi")
	newNode.Status.Conditions = []v1.NodeCondition{{Type: v1.NodeReady, Status: v1.ConditionFalse}}
	mockKubernetesClient.Nodes[newNode.Name] = newNode
	HandleRollingUpgrade(mockKubernetesClient, mockEc2Service, mockAutoScalingService, []*autoscaling.Group{asg})
	oldNode = mockKubernetesClient.Nodes[aws.StringValue(oldInstance.InstanceId)]
	if _, ok := oldNode.GetAnnotations()[k8s.RollingUpdateDrainedTimestampAnnotationKey]; ok {
		t.Error("Node shouldn't have been drained yet, therefore shouldn't have been annotated with", k8s.RollingUpdateDrainedTimestampAnnotationKey)
	}

	// Seventh run (Kubelet is ready to accept new pods. Old node gets drained and terminated)
	newNode = mockKubernetesClient.Nodes[newNode.Name]
	newNode.Status.Conditions = []v1.NodeCondition{{Type: v1.NodeReady, Status: v1.ConditionTrue}}
	mockKubernetesClient.Nodes[newNode.Name] = newNode
	HandleRollingUpgrade(mockKubernetesClient, mockEc2Service, mockAutoScalingService, []*autoscaling.Group{asg})
	oldNode = mockKubernetesClient.Nodes[aws.StringValue(oldInstance.InstanceId)]
	if _, ok := oldNode.GetAnnotations()[k8s.RollingUpdateDrainedTimestampAnnotationKey]; !ok {
		t.Error("Node should've been drained")
	}
	if _, ok := oldNode.GetAnnotations()[k8s.RollingUpdateTerminatedTimestampAnnotationKey]; !ok {
		t.Error("Node should've been terminated")
	}
}
