export function Footer() {
  return (
    <footer className="footer">
      <div className="container footer__inner">
        <a
          className="footer__copy"
          href="https://github.com/z08w12/omni-media-vault"
          target="_blank"
          rel="noopener noreferrer"
        >
          © {new Date().getFullYear()} 91
        </a>
      </div>
    </footer>
  );
}
