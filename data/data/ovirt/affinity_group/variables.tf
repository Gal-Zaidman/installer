variable "ovirt_cluster_id" {
  type        = string
  description = "The ID of Cluster"
}

variable "ovirt_masters_affinity_groups_names" {
  type        = list(string)
  description = "Name of the masters affinity groups that will be created."
  default     = []
}

variable "ovirt_cluster_affinity_groups_names" {
  type        = list(string)
  description = "Name of the cluster affinity groups that will be created."
  default     = []
}


variable "masters_ids" {
  type        = list(string)
  description = "Name of the Affinity Group that will be created."
}
