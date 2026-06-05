/**
 * Helpers for merging Socket.IO / REST update payloads into existing
 * Tabulator row data.
 *
 * OpenAPI models omit JSON keys instead of sending `null`, so a shallow
 * `Object.assign` would clobber existing cells with `undefined`. The helpers
 * below perform a shallow merge that skips `undefined`, so omitted keys do
 * not overwrite previously-known values.
 */

/**
 * Shallow-merge a task's `worker` field, ignoring `undefined` patches.
 *
 * @param {object|null|undefined} existingWorker - Worker on the existing row, if any.
 * @param {object|null|undefined} patchWorker    - Worker from the update payload.
 * @returns {object|null|undefined} The merged worker:
 *   - `existingWorker` when the patch is `undefined` (worker key omitted),
 *   - `null` when the patch explicitly clears the worker,
 *   - a shallow `{ ...existing, ...patch }` otherwise.
 */
export function mergeWorkerField(existingWorker, patchWorker) {
  if (patchWorker === undefined) {
    return existingWorker;
  }
  if (patchWorker === null) {
    return null;
  }
  const base = existingWorker || {};
  return { ...base, ...patchWorker };
}

/**
 * Merge a task update payload into an existing Tabulator row.
 *
 * Keys with an `undefined` value in `update` are skipped, so OpenAPI-omitted
 * fields do not clobber previously-known cells. The nested `worker` object
 * is merged with `mergeWorkerField`.
 *
 * @param {object|null|undefined} existing - Existing row data, or `null` if the row is new.
 * @param {object} update                  - Task update payload (REST or Socket.IO shape).
 * @returns {object} A new row object with the patch applied.
 */
export function mergeTaskRow(existing, update) {
  if (!existing) {
    return taskRowFromUpdate(update);
  }
  const merged = { ...existing };
  for (const key of Object.keys(update)) {
    const value = update[key];
    if (value === undefined) {
      continue;
    }
    if (key === 'worker') {
      merged.worker = mergeWorkerField(existing.worker, value);
    } else {
      merged[key] = value;
    }
  }
  return merged;
}

/**
 * Build a fresh task row object from an update payload, skipping `undefined`
 * keys so omitted fields stay absent rather than appearing as `undefined`.
 *
 * @param {object} update - Task update payload.
 * @returns {object} A new row object containing only the defined fields.
 */
function taskRowFromUpdate(update) {
  const row = {};
  for (const key of Object.keys(update)) {
    const value = update[key];
    if (value === undefined) {
      continue;
    }
    row[key] = value;
  }
  return row;
}

/**
 * Merge a job update payload into an existing Tabulator row.
 *
 * Keys with an `undefined` value in `update` are skipped, so OpenAPI-omitted
 * fields do not clobber previously-known cells.
 *
 * @param {object} existing - Existing row data.
 * @param {object} update   - Job update payload (REST or Socket.IO shape).
 * @returns {object} A new row object with the patch applied.
 */
export function mergeJobRow(existing, update) {
  console.assert(existing, 'mergeJobRow requires existing row data');
  const merged = { ...existing };
  for (const key of Object.keys(update)) {
    const value = update[key];
    if (value === undefined) {
      continue;
    }
    merged[key] = value;
  }
  return merged;
}
