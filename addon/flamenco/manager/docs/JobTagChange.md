# JobTagChange

New tag to assign to a job. Can be empty to remove the tag (and thus make the job available to all workers).

## Properties
Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**id** | **str** | UUID of the tag. If this is given, &#39;name&#39; should not be given. | [optional] 
**name** | **str** | Name of the tag. If this is given, &#39;id&#39; should not be given. | [optional] 
**any string name** | **bool, date, datetime, dict, float, int, list, str, none_type** | any string name can be used but the value must be the correct type | [optional]

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


