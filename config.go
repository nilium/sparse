package sparse

type Configuration interface {
	apply(*Parser)
}

type ReadComments bool

func (b ReadComments) apply(p *Parser) { p.readComments = bool(b) }

type TrimWhitespace bool

func (b TrimWhitespace) apply(p *Parser) { p.keepTrailingWhitespace = !bool(b) }

type CompressWhitespace bool

func (b CompressWhitespace) apply(p *Parser) { p.keepSeqWhitespace = !bool(b) }
