package main

/*

#cgo pkg-config: python3
#include <Python.h>
#include <stdbool.h>

// Workaround missing variadic function support
// https://github.com/golang/go/issues/975
int PyArg_ParseTuple_run(PyObject * args, char **a, char **b, char **c, bool *d, char **e) {
    return PyArg_ParseTuple(args, "sssps", a, b, c, d, e);
}

int PyArg_ParseTuple_add_voter(PyObject * args, char **a, char **b, char **c) {
    return PyArg_ParseTuple(args, "sss", a, b, c);
}

PyObject * run(PyObject* , PyObject*);

PyObject * add_voter(PyObject* , PyObject*);

static PyObject *AddVoterError = NULL;

static PyMethodDef methods[] = {
    {"run", (PyCFunction)run, METH_VARARGS, "Run the raft Node server"},
    {"add_voter", (PyCFunction)add_voter, METH_VARARGS, "Client to add voter"},
    {NULL, NULL, 0, NULL}
};

static struct PyModuleDef jraftmodule = {
   PyModuleDef_HEAD_INIT, "jraft", NULL, -1, methods
};

PyMODINIT_FUNC
PyInit_jraft(void)
{
    PyObject *m;
    m = PyModule_Create(&jraftmodule);
    if (m == NULL)
        return NULL;

    AddVoterError = PyErr_NewException("jraft.AddVoterError", PyExc_RuntimeError, NULL);
    Py_XINCREF(AddVoterError);

    if (PyModule_AddObject(m, "error", AddVoterError) < 0) {
        Py_XDECREF(AddVoterError);
        Py_CLEAR(AddVoterError);
        Py_DECREF(m);
        return NULL;
    }

    return m;
}

void raise_exception(char *msg) {
    PyErr_SetString(PyExc_ValueError, msg);
    return NULL;
}

*/
import "C"


