FROM python:3.10-alpine

RUN apk add git build-base libffi-dev scons patchelf
ENV CRYPTOGRAPHY_DONT_BUILD_RUST=1
RUN pip install poetry pyinstaller psycopg2-binary staticx

WORKDIR /app

RUN git clone --depth=1 -b feature/privileges https://github.com/stack11/schemainspect && \
    cd schemainspect && \
    poetry build && \
    pip install dist/schemainspect-*-none-any.whl

RUN git clone --depth=1 https://github.com/djrobstep/sqlbag && \
    cd sqlbag && \
    pip install .

RUN git clone --depth=1 https://github.com/djrobstep/migra && \
    cd migra && \
    printf "from migra.command import do_command\ndo_command()" > main.py && \
    pyinstaller main.py --onefile --name migra --collect-data schemainspect && \
    mkdir /app/out && \
    staticx dist/migra /app/out/migra
