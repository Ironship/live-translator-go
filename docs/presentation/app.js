(() => {
  const slides = [...document.querySelectorAll('.slide')];
  const dots = [...document.querySelectorAll('.dots a')];
  const deck = document.querySelector('.deck');

  // Active dot on scroll
  const io = new IntersectionObserver((entries) => {
    entries.forEach(e => {
      if (e.isIntersecting) {
        const id = e.target.id;
        dots.forEach(d => d.classList.toggle('active', d.getAttribute('href') === '#' + id));
      }
    });
  }, { root: deck, threshold: 0.55 });
  slides.forEach(s => io.observe(s));

  // Keyboard navigation
  const go = (dir) => {
    const cur = slides.findIndex(s => {
      const r = s.getBoundingClientRect();
      return r.top >= -window.innerHeight / 2 && r.top < window.innerHeight / 2;
    });
    const next = Math.max(0, Math.min(slides.length - 1, (cur < 0 ? 0 : cur) + dir));
    slides[next].scrollIntoView({ behavior: 'smooth' });
  };
  window.addEventListener('keydown', (e) => {
    if (['ArrowDown', 'PageDown', ' ', 'Enter'].includes(e.key)) { e.preventDefault(); go(1); }
    else if (['ArrowUp', 'PageUp'].includes(e.key)) { e.preventDefault(); go(-1); }
    else if (e.key === 'Home') { e.preventDefault(); slides[0].scrollIntoView({ behavior: 'smooth' }); }
    else if (e.key === 'End') { e.preventDefault(); slides[slides.length - 1].scrollIntoView({ behavior: 'smooth' }); }
  });
})();
