// iOS only opens the keyboard for a focus() made during the tap itself, and
// the form mounts a moment later. Focusing a stand-in input on the tap opens
// the keyboard, and it stays open when the form moves focus to its own field.
export function openKeyboard() {
  const proxy = document.createElement('input');
  proxy.setAttribute('aria-hidden', 'true');
  proxy.tabIndex = -1;
  // 16px keeps iOS from zooming in
  proxy.style.cssText = 'position:fixed;top:0;left:0;width:1px;height:1px;opacity:0;font-size:16px;border:0;padding:0';
  document.body.appendChild(proxy);
  proxy.focus();
  const done = () => proxy.remove();
  proxy.addEventListener('blur', done, { once: true });
  setTimeout(done, 1000);
}
