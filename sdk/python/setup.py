from setuptools import setup, find_packages

setup(
    name="deploysentry",
    version="0.1.0",
    description="Official Python SDK for the DeploySentry platform",
    long_description=open("README.md").read(),
    long_description_content_type="text/markdown",
    author="DeploySentry",
    author_email="sdk@deploysentry.io",
    url="https://github.com/deploysentry/deploysentry",
    project_urls={
        "Documentation": "https://docs.deploysentry.io/sdk/python",
        "Source": "https://github.com/deploysentry/deploysentry/tree/main/sdk/python",
        "Tracker": "https://github.com/deploysentry/deploysentry/issues",
    },
    packages=find_packages(where="src"),
    package_dir={"": "src"},
    python_requires=">=3.9",
    install_requires=[
        "httpx>=0.24.0",
    ],
    extras_require={
        "dev": [
            "pytest>=7.0.0",
            "pytest-asyncio>=0.21.0",
            "ruff>=0.1.0",
        ],
    },
    classifiers=[
        "Development Status :: 3 - Alpha",
        "Intended Audience :: Developers",
        "License :: OSI Approved :: Apache Software License",
        "Programming Language :: Python :: 3",
        "Programming Language :: Python :: 3.9",
        "Programming Language :: Python :: 3.10",
        "Programming Language :: Python :: 3.11",
        "Programming Language :: Python :: 3.12",
    ],
    license="Apache-2.0",
)
