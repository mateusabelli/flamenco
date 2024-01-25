---
title: Tags
---

Sometimes you want **a job to be sent only to some workers, but not all of them**.
This can be due to memory requirements of that job, the GPU the workers have
available, or any other reason. This is what you can use tags for.

## How does it work?

How this works is easiest to explain when we look at two perspectives:

### From the perspective of the job

- A job can have one tag, or no tag.
- A job **with** a tag will only be assigned to workers with that tag.
- A job **without** tag will be assigned to any worker.

### From the perspective of the worker

- A worker can have any number of tags.
- A worker **with** one or more tags will work only on jobs with one those tags, and on tagless jobs.
- A worker **without** tags will only work on tagless jobs.


## Example: Blender Studio

[Blender Studio](https://studio.blender.org/) have two groups of Workers:

- **Artist machines**, with powerful GPUs. These are suitable for EEVEE renders, but
  also Cycles-on-GPU, and can also help with Cycles-on-CPU jobs.
- **Render servers**, with lots of CPU power, but no GPUs. These can only do
  Cycles-on-CPU jobs.

To support these different cases, they use three tags:

- `EEVEE`
- `Cycles`
- `Cycles GPU`

The **artist machines** get all three tags. The **render servers** just get the
`Cycles` tag. When submitting a job, the artists will chose the tag that is
suitable for that particular job.

{{< hint >}}
Choosing the tag for a job is something you have to do yourself. In this example
case, the tag could in theory be picked automatically depending on the active
render engine. The tagging system is more general than that, though, and so
Flamenco doesn't know what *you* want to use the tags for.
{{< /hint >}}

For more info on GPU rendering with Flamenco, see [FAQ: How do I make the
Workers render on GPU?][faq-gpu]

[faq-gpu]:  {{< ref "faq" >}}#how-do-i-make-the-workers-render-on-gpu
