.PHONY: install dev-login dev-serve test clean

install:
	python -m pip install -e .

dev-login:
	python -m claude_bridge login

dev-serve:
	python -m claude_bridge serve

test:
	python -m compileall claude_bridge

clean:
	rm -rf build dist *.egg-info
	find claude_bridge -type d -name __pycache__ -prune -exec rm -rf {} +
