/**
 * This debounce function delays the call of a custom function, and resets the timer each time it is called.
 *
 * It is useful for limiting the amount of calls, particularly within event listeners on high frequency events
 * (e.g. keystrokes on a text input, or page scrolling).
 *
 * For example, if debounce is called in a watcher for a text input, then on a keystroke, a timer for 250ms will start,
 * and fn will be executed after 250ms after that keystroke. But if another keystroke happens again before the timer gets to 0,
 * it will interrupt that timer, reset it to 250ms and fn will only get executed when the new timer reaches 0, and so on.
 * @param {*} fn the custom function
 * @param {*} validation_timeout_handle the timer ID
 * @returns the new timer ID
 */
export const debounce = (fn, validation_timeout_handle) => {
  if (validation_timeout_handle) clearTimeout(validation_timeout_handle);

  return setTimeout(() => {
    fn();
  }, 250); // delay by 250ms
};
