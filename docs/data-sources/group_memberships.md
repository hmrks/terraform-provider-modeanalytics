---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "modeanalytics_group_memberships Data Source - modeanalytics"
subcategory: ""
description: |-
  Data source for retrieving member tokens of a group
---

# modeanalytics_group_memberships (Data Source)

Data source for retrieving member tokens of a group



<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `group_token` (String) The token identifying the group.

### Read-Only

- `member_tokens` (List of String) A list of member tokens in the group.
