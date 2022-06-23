FROM python:3.10-alpine

RUN apk add git build-base libffi-dev scons patchelf
ENV CRYPTOGRAPHY_DONT_BUILD_RUST=1
RUN pip install poetry pyinstaller psycopg2-binary staticx

WORKDIR /app

RUN git clone https://github.com/biodevc/schemainspect && \
    cd schemainspect && \
    git checkout 3689c45 && \
    poetry build && \
    pip install dist/schemainspect-*-none-any.whl

RUN git clone https://github.com/djrobstep/sqlbag && \
    cd sqlbag && \
    git checkout eaaeec4 && \
    pip install .

RUN git clone https://github.com/djrobstep/migra && \
    cd migra && \
    git checkout 5d7d2f4 && \
    printf "from migra.command import do_command\ndo_command()" > main.py && \
    pyinstaller main.py --onefile --name migra --collect-data schemainspect && \
    mkdir /app/out && \
    staticx dist/migra /app/out/migra
