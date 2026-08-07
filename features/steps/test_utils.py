import pathlib
import shutil

from behave import step
import behave.runner

@step("file '{src_file}' is installed in '{target_dir}'")
def step_impl(context: behave.runner.Context, src_file, target_dir):
    """
    Copies a file from the source location to the target directory.

    :param context: The context object.
    :param src_file: The source file path.
    :param target_dir: The target directory path.
    :return: None
    """
    shutil.copy(src_file, target_dir)


@step("file '{src_file}' is deleted")
def step_impl(context: behave.runner.Context, src_file):
    """
    Removes a file from the target directory.

    :param context: The context object.
    :param src_file: The source file path.
    :return: None
    """
    pathlib.Path(src_file).unlink()
