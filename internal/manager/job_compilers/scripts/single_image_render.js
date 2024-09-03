const JOB_TYPE = {
  label: "Single Image Render",
  description: "Distributed rendering of a single image.",
  settings: [
    // Settings for artists to determine:
    {
      key: "tile_size_x",
      type: "int32",
      default: 64,
      description: "Tile size in pixels for the X axis"
    },
    {
      key: "tile_size_y",
      type: "int32",
      default: 64,
      description: "Tile size in pixels for the Y axis"
    },
    {
      key: "frame", type: "int32", required: true,
      eval: "C.scene.frame_current",
      description: "Frame to render. Examples: '47', '1'"
    },

    // render_output_root + add_path_components determine the value of render_output_path.
    {
      key: "render_output_root",
      type: "string",
      subtype: "dir_path",
      required: true,
      visible: "submission",
      description: "Base directory of where render output is stored. Will have some job-specific parts appended to it"
    },
    {
      key: "add_path_components",
      type: "int32",
      required: true,
      default: 0,
      propargs: {min: 0, max: 32},
      visible: "submission",
      description: "Number of path components of the current blend file to use in the render output path"
    },
    {
      key: "render_output_path", type: "string", subtype: "file_path", editable: false,
      eval: "str(Path(abspath(settings.render_output_root), last_n_dir_parts(settings.add_path_components), jobname, '{timestamp}', 'tiles'))",
      description: "Final file path of where render output will be saved"
    },

    // Automatically evaluated settings:
    {
      key: "blendfile",
      type: "string",
      required: true,
      description: "Path of the Blend file to render",
      visible: "web"
    },
    {
      key: "format",
      type: "string",
      required: true,
      eval: "C.scene.render.image_settings.file_format",
      visible: "web"
    },
    {
      key: "image_file_extension",
      type: "string",
      required: true,
      eval: "C.scene.render.file_extension",
      visible: "hidden",
      description: "File extension used when rendering images"
    },
    {
      key: "resolution_x",
      type: "int32",
      required: true,
      eval: "C.scene.render.resolution_x",
      visible: "hidden",
      description: "Resolution X"
    },
    {
      key: "resolution_y",
      type: "int32",
      required: true,
      eval: "C.scene.render.resolution_y",
      visible: "hidden",
      description: "Resolution Y"
    },
    {
      key: "resolution_scale",
      type: "int32",
      required: true,
      eval: "C.scene.render.resolution_percentage",
      visible: "hidden",
      description: "Resolution scale"
    }
  ]
};

function compileJob(job) {
  print("Single Image Render job submitted");
  print("job: ", job);

  const settings = job.settings;
  const renderOutput = renderOutputPath(job);

  if (settings.resolution_scale !== 100) {
    throw "Flamenco currently does not support rendering with a resolution scale other than 100%";
  }

  // Make sure that when the job is investigated later, it shows the
  // actually-used render output:
  settings.render_output_path = renderOutput;

  const renderDir = path.dirname(renderOutput);
  const renderTasks = authorRenderTasks(settings, renderDir, renderOutput);
  const mergeTask = authorMergeTask(settings, renderDir);

  for (const rt of renderTasks) {
    job.addTask(rt);
  }
  if (mergeTask) {
    // If there is a merge task, all other tasks have to be done first.
    for (const rt of renderTasks) {
      mergeTask.addDependency(rt);
    }
    job.addTask(mergeTask);
  }
}

// Do field replacement on the render output path.
function renderOutputPath(job) {
  let path = job.settings.render_output_path;
  if (!path) {
    throw "no render_output_path setting!";
  }
  return path.replace(/{([^}]+)}/g, (match, group0) => {
    switch (group0) {
      case "timestamp":
        return formatTimestampLocal(job.created);
      default:
        return match;
    }
  });
}

// Calculate the borders for the tiles
// Does not take into account the overscan
function calcBorders(tileSizeX, tileSizeY, width, height) {
  let borders = [];
  for (let y = 0; y < height; y += tileSizeY) {
    for (let x = 0; x < width; x += tileSizeX) {
      borders.push([x, y, Math.min(x + tileSizeX, width), Math.min(y + tileSizeY, height)]);
    }
  }
  print("borders: ", borders);
  return borders;
}

function authorRenderTasks(settings, renderDir, renderOutput) {
  print("authorRenderTasks(", renderDir, renderOutput, ")");
  let renderTasks = [];
  let borders = calcBorders(settings.tile_size_x, settings.tile_size_y, settings.resolution_x, settings.resolution_y);
  for (let border of borders) {
    const task = author.Task(`render-${border[0]}-${border[1]}`, "blender");
    // Overscan is calculated in this manner to avoid rendering outside the image resolution
    let pythonExpr = `import bpy

scene = bpy.context.scene
render = scene.render
render.image_settings.file_format = 'OPEN_EXR_MULTILAYER'
render.use_compositing = False
render.use_stamp = False
overscan = 16

render.border_min_x = max(${border[0]} - overscan, 0) / ${settings.resolution_x}
render.border_min_y = max(${border[1]} - overscan, 0) / ${settings.resolution_y}
render.border_max_x = min(${border[2]} + overscan, ${settings.resolution_x}) / ${settings.resolution_x}
render.border_max_y = min(${border[3]} + overscan, ${settings.resolution_x}) / ${settings.resolution_y}
render.use_border = True
render.use_crop_to_border = True
bpy.ops.render.render(write_still=True)`
    const command = author.Command("blender-render", {
      exe: "{blender}",
      exeArgs: "{blenderArgs}",
      argsBefore: [],
      blendfile: settings.blendfile,
      args: [
        "--render-output", path.join(renderDir, path.basename(renderOutput), border[0] + "-" + border[1] + "-" + border[2] + "-" + border[3]),
        "--render-format", settings.format,
        "--python-expr", pythonExpr
      ]
    });
    task.addCommand(command);
    renderTasks.push(task);
  }
  return renderTasks;
}

function authorMergeTask(settings, renderDir, renderOutput) {
  print("authorMergeTask(", renderDir, ")");
  const task = author.Task("merge", "blender");
  // Burning metadata into the image is done by the compositor for the entire merged image
  // The overall logic of the merge is as follows:
  // 1. Find out the Render Layers node and to which socket it is connected
  // 2. Load image files from the tiles directory.
  //    Their correct position is determined by their filename.
  // 3. Create a node tree that scales, translates and adds the tiles together.
  //    A simple version of the node tree is linked here:
  //    https://devtalk.blender.org/uploads/default/original/3X/f/0/f047f221c70955b32e4b455e53453c5df716079e.jpeg
  // 4. The final image is then fed into the socket the Render Layers node was connected to.
  //    This allows the compositing to work as if the image was rendered in one go.
  let pythonExpr = `import bpy

render = bpy.context.scene.render
render.resolution_x = ${settings.resolution_x}
render.resolution_y = ${settings.resolution_y}
bpy.context.scene.use_nodes = True
render.use_compositing = True
render.use_stamp = True
node_tree = bpy.context.scene.node_tree
overscan = 16

render_layers_node = None
for node in node_tree.nodes:
    if node.type == 'R_LAYERS':
        feed_in_input = node.outputs[0]
        render_layers_node = node
        break
for link in node_tree.links:
    if feed_in_input is not None and link.from_socket == feed_in_input:
        feed_in_output = link.to_socket
        break

from pathlib import Path

root = Path("${path.join(renderDir, path.basename(renderOutput))}/tiles")
image_files = [f for f in root.iterdir() if f.is_file()]

separate_nodes = []
first_crop_node = None
translate_nodes = []
min_width = min([int(f.stem.split('-')[2]) - int(f.stem.split('-')[0]) for f in image_files])
min_height = min([int(f.stem.split('-')[3]) - int(f.stem.split('-')[1]) for f in image_files])
for i, image_file in enumerate(image_files):
    image_node = node_tree.nodes.new('CompositorNodeImage')
    image_node.image = bpy.data.images.load(str(root / image_file.name))

    crop_node = node_tree.nodes.new('CompositorNodeCrop')
    crop_node.use_crop_size = True
    left, top, right, bottom = image_file.stem.split('-')
    actual_width = int(right) - int(left)
    actual_height = int(bottom) - int(top)
    if left == '0':
        crop_node.min_x = 0
        crop_node.max_x = actual_width
    else:
        crop_node.min_x = overscan
        crop_node.max_x = actual_width + overscan
    if top == '0':
        crop_node.max_y = 0
        crop_node.min_y = actual_height
    else:
        crop_node.max_y = overscan
        crop_node.min_y = actual_height + overscan
    if i == 0:
        first_crop_node = crop_node

    translate_node = node_tree.nodes.new('CompositorNodeTranslate')
    # translate_node.use_relative = True
    translate_node.inputs[1].default_value = float(left) + (actual_width - ${settings.resolution_x}) / 2
    translate_node.inputs[2].default_value = float(top) + (actual_height - ${settings.resolution_y}) / 2
    translate_nodes.append(translate_node)

    separate_node = node_tree.nodes.new('CompositorNodeSeparateColor')
    separate_nodes.append(separate_node)

    node_tree.links.new(image_node.outputs[0], crop_node.inputs[0])
    node_tree.links.new(crop_node.outputs[0], translate_node.inputs[0])
    node_tree.links.new(translate_node.outputs[0], separate_node.inputs[0])

scale_node = node_tree.nodes.new('CompositorNodeScale')
scale_node.space = 'RELATIVE'
scale_node.inputs[1].default_value = ${settings.resolution_x} / min_width
scale_node.inputs[2].default_value = ${settings.resolution_y} / min_height
node_tree.links.new(first_crop_node.outputs[0], scale_node.inputs[0])
mix_node = node_tree.nodes.new('CompositorNodeMixRGB')
mix_node.blend_type = 'MIX'
mix_node.inputs[0].default_value = 0.0
mix_node.inputs[1].default_value = (0, 0, 0, 1)
node_tree.links.new(scale_node.outputs[0], mix_node.inputs[2])

mix_adds = [node_tree.nodes.new('CompositorNodeMixRGB') for _ in range(len(separate_nodes))]
math_adds = [node_tree.nodes.new('CompositorNodeMath') for _ in range(len(separate_nodes))]
for i, mix_add in enumerate(mix_adds):
    mix_add.blend_type = 'ADD'
    if i == 0:
        node_tree.links.new(mix_node.outputs[0], mix_add.inputs[1])
    else:
        node_tree.links.new(mix_adds[i - 1].outputs[0], mix_add.inputs[1])
    node_tree.links.new(translate_nodes[i].outputs[0], mix_add.inputs[2])

for i, math_add in enumerate(math_adds):
    math_add.operation = 'ADD'
    if i == 0:
        node_tree.links.new(mix_node.outputs[0], math_add.inputs[0])
    else:
        node_tree.links.new(math_adds[i - 1].outputs[0], math_add.inputs[0])
    node_tree.links.new(separate_nodes[i - 1].outputs[3], math_add.inputs[1])

set_alpha_node = node_tree.nodes.new('CompositorNodeSetAlpha')
set_alpha_node.mode = 'REPLACE_ALPHA'
node_tree.links.new(mix_adds[-1].outputs[0], set_alpha_node.inputs[0])
node_tree.links.new(math_adds[-1].outputs[0], set_alpha_node.inputs[1])
if feed_in_input is not None:
    node_tree.links.new(set_alpha_node.outputs[0], feed_in_output)
else:
    raise Exception('No Render Layers Node found. Currently only supported with a Render Layers Node in the Compositor.')

node_tree.nodes.remove(render_layers_node)
bpy.ops.render.render(write_still=True)`

  const command = author.Command("blender-render", {
    exe: "{blender}",
    exeArgs: "{blenderArgs}",
    argsBefore: [],
    blendfile: settings.blendfile,
    args: [
      "--render-output", path.join(renderDir, path.basename(renderOutput), "merged"),
      "--render-format", settings.format,
      "--python-expr", pythonExpr
    ]
  });
  task.addCommand(command);
  return task;
}
