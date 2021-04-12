output "releaseimage_template_id" {
  value = var.bootstrap?data.ovirt_templates.finalTemplate.templates.0.id:""
}

output "tmp_import_vm" {
  value = var.bootstrap?(length(ovirt_vm.tmp_import_vm) > 0 ? ovirt_vm.tmp_import_vm.0.id : ""):""
}
