resource "ovirt_affinity_group" "cluster_masters_affinity_group" {
  count        = length(var.ovirt_masters_affinity_groups_names)
  name         = var.ovirt_masters_affinity_groups_names[count.index]
  cluster_id   = var.ovirt_cluster_id
  priority     = 5
  vm_positive  = false
  vm_enforcing = false
  vm_list      = var.masters_ids
}

resource "ovirt_affinity_group" "cluster_affinity_group" {
  count        = length(var.ovirt_cluster_affinity_groups_names)
  name         = var.ovirt_cluster_affinity_groups_names[count.index]
  cluster_id   = var.ovirt_cluster_id
  priority     = 2
  vm_positive  = false
  vm_enforcing = false
  vm_list      = var.masters_ids
}