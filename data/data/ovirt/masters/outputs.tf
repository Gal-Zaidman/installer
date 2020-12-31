output "masters_ids" {
  value = ovirt_vm.master.*.id
}
