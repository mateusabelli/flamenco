# Command

Command represents a single command to execute by the Worker.

## Properties
Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**name** | **str** |  | 
**parameters** | **{str: (bool, date, datetime, dict, float, int, list, str, none_type)}** |  | 
**total_step_count** | **int** | Number of steps this command executes. This has to be implemented in the command&#39;s implementation on the Worker (to recognise what a \&quot;step\&quot; is), as well as given in the authoring code of the job type JavaScript script (to indicate how many steps the command invocation will perform). If not given, or set to 0, the command is not expected to send any step progress. In this case, the Worker will send a step update at completion of the command.  | 
**any string name** | **bool, date, datetime, dict, float, int, list, str, none_type** | any string name can be used but the value must be the correct type | [optional]

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


